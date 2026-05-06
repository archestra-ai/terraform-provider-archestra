#!/usr/bin/env bash
#
# Brings up the full Archestra dev stack the same way CI does — kind cluster,
# helm-installed platform chart, ollama mock + dev Vault deployed as K8s
# manifests under the same kubectl context. Pinned to ARCHESTRA_VERSION.
#
# Why this shape: the platform's local MCP server installer (`MCPServerRuntimeManager`)
# expects a real K8s API to create deployments. The previous "single docker
# container with docker.sock mounted" version couldn't reliably deploy MCP
# servers, surfacing as `waitForServerTools` timeouts on cold starts.
#
# Required tools: docker, kind, kubectl, helm, curl, jq.
#
# Re-run is idempotent: cluster reused if present, helm uses upgrade --install,
# kubectl apply is idempotent for the manifests.
#
# After it exits successfully:
#   eval "$(scripts/bootstrap-local-stack.sh --print-env)"
#   make testacc
#
# Print the env-var snippet without touching the cluster:
#   scripts/bootstrap-local-stack.sh --print-env
#
# Tear it all down (deletes the kind cluster):
#   scripts/bootstrap-local-stack.sh --down

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

KIND_CLUSTER="${KIND_CLUSTER:-archestra-ci-cluster}"
KIND_CONFIG="${REPO_ROOT}/.github/kind.yaml"
HELM_VALUES="${REPO_ROOT}/.github/values-ci.yaml"
HELM_RELEASE="${HELM_RELEASE:-archestra-platform}"
HELM_CHART_OCI="${HELM_CHART_OCI:-oci://europe-west1-docker.pkg.dev/friendly-path-465518-r6/archestra-public/helm-charts/archestra-platform}"

log() { printf '%s\n' "$*" >&2; }

# resolve_version reads ARCHESTRA_VERSION from CI's source of truth so local
# tracks CI without re-hardcoding a tag here. Only called on the `up` path —
# --down and --print-env don't need it.
resolve_version() {
  if [ -n "${ARCHESTRA_VERSION:-}" ]; then
    return
  fi
  ARCHESTRA_VERSION=$(grep -E '^\s*ARCHESTRA_VERSION:\s*"' \
    "${REPO_ROOT}/.github/workflows/on-pull-request.yml" 2>/dev/null \
    | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$ARCHESTRA_VERSION" ]; then
    log "Could not resolve ARCHESTRA_VERSION (set it explicitly or fix .github/workflows/on-pull-request.yml)"
    exit 1
  fi
}

require_tool() {
  command -v "$1" >/dev/null 2>&1 || {
    log "Missing required tool: $1. Install instructions:"
    case "$1" in
      kind) log "  https://kind.sigs.k8s.io/docs/user/quick-start/#installation" ;;
      helm) log "  https://helm.sh/docs/intro/install/" ;;
      kubectl) log "  https://kubernetes.io/docs/tasks/tools/" ;;
      *) log "  ($1)" ;;
    esac
    exit 1
  }
}

print_env() {
  cat <<EOF
export ARCHESTRA_BASE_URL=http://localhost:9000
export ARCHESTRA_API_KEY=\$(${SCRIPT_DIR}/bootstrap-api-key.sh)
export ARCHESTRA_READONLY_VAULT_ENABLED=true
export ARCHESTRA_TEST_IDP_ID=\$(ARCHESTRA_API_KEY=\$ARCHESTRA_API_KEY ${SCRIPT_DIR}/bootstrap-test-idp.sh)
export TF_ACC=1
EOF
}

down() {
  log "Deleting kind cluster ${KIND_CLUSTER}..."
  kind delete cluster --name "$KIND_CLUSTER" 2>/dev/null || true
  log "Down."
}

case "${1:-}" in
  --print-env) print_env; exit 0 ;;
  --down) down; exit 0 ;;
esac

require_tool docker
require_tool kind
require_tool kubectl
require_tool helm
require_tool curl
require_tool jq

resolve_version
log "Bringing up Archestra ${ARCHESTRA_VERSION} local stack on kind..."

# 1. Kind cluster — reuse if already present (matches CI's name + ports).
if ! kind get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER"; then
  log "Creating kind cluster ${KIND_CLUSTER}..."
  kind create cluster --config "$KIND_CONFIG" --name "$KIND_CLUSTER"
else
  log "Kind cluster ${KIND_CLUSTER} already exists, reusing."
fi

# Switch kubectl context to the kind cluster so subsequent kubectl/helm
# commands target the right cluster regardless of the user's previous context.
kubectl config use-context "kind-${KIND_CLUSTER}" >/dev/null

# 2. Pull platform image into the local Docker daemon and load into kind.
PLATFORM_IMAGE="archestra/platform:${ARCHESTRA_VERSION}"
if ! docker image inspect "$PLATFORM_IMAGE" >/dev/null 2>&1; then
  log "Pulling ${PLATFORM_IMAGE}..."
  docker pull "$PLATFORM_IMAGE" >/dev/null
fi
log "Loading ${PLATFORM_IMAGE} into kind..."
kind load docker-image "$PLATFORM_IMAGE" --name "$KIND_CLUSTER" >/dev/null

# 3. Helm install/upgrade the platform chart with the same values CI uses.
log "Helm installing ${HELM_RELEASE} (chart ${ARCHESTRA_VERSION})..."
helm upgrade --install "$HELM_RELEASE" "$HELM_CHART_OCI" \
  --version "$ARCHESTRA_VERSION" \
  --values "$HELM_VALUES" \
  --set "archestra.image=${PLATFORM_IMAGE}" \
  --atomic --timeout=5m >/dev/null

# 4. Wait for platform pods to be ready before deploying the dependencies
#    that the platform's healthcheck doesn't gate on.
log "Waiting for platform pods..."
kubectl wait --for=condition=Ready pods -l app.kubernetes.io/name=archestra-platform --timeout=120s >/dev/null

# 5. Ollama mock + dev Vault — same manifests CI applies post-platform.
"${SCRIPT_DIR}/bootstrap-ollama-mock.sh"
"${SCRIPT_DIR}/bootstrap-vault.sh"

# 6. Backend health gate — kind's NodePort 30000 is mapped to host 9000 by
#    .github/kind.yaml. /api/health returning 200 or 401 means the API is up.
log "Waiting for backend at http://localhost:9000..."
deadline=$(($(date +%s) + 120))
while :; do
  c=$(curl -sS --connect-timeout 2 --max-time 3 -o /dev/null -w '%{http_code}' \
        http://localhost:9000/api/health 2>/dev/null || echo "")
  case "$c" in 200|401) break ;; esac
  [ "$(date +%s)" -lt "$deadline" ] || { log "backend never became reachable"; exit 1; }
  sleep 2
done

# 7. Seed an Ollama provider key so the LLM model tests find at least one model.
api_key=$("${SCRIPT_DIR}/bootstrap-api-key.sh")
existing=$(curl -sS --connect-timeout 5 --max-time 10 -H "Authorization: $api_key" \
  http://localhost:9000/api/llm-provider-api-keys \
  | jq -r 'map(select(.name=="local-stack-ollama")) | (.[0].id // empty)')
if [ -z "$existing" ]; then
  curl -sS --connect-timeout 5 --max-time 10 -X POST \
    http://localhost:9000/api/llm-provider-api-keys \
    -H "Authorization: $api_key" -H "Content-Type: application/json" \
    -d '{"name":"local-stack-ollama","provider":"ollama","vaultSecretPath":"secret/data/test/ollama","vaultSecretKey":"api_key","scope":"org"}' >/dev/null
fi

# 8. Provision the EMC test IdP.
ARCHESTRA_API_KEY="$api_key" "${SCRIPT_DIR}/bootstrap-test-idp.sh" >/dev/null

log "Stack ready."
log
log "  eval \"\$(scripts/bootstrap-local-stack.sh --print-env)\" && make testacc"
log
log "Tear down with:  scripts/bootstrap-local-stack.sh --down"
