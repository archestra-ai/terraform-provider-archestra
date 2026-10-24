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

BACKEND_PORT="${ARCHESTRA_TEST_BACKEND_PORT:-9000}"
FRONTEND_PORT="${ARCHESTRA_TEST_FRONTEND_PORT:-3000}"
METRICS_PORT="${ARCHESTRA_TEST_METRICS_PORT:-9050}"
for port in "$BACKEND_PORT" "$FRONTEND_PORT" "$METRICS_PORT"; do
  [[ "$port" =~ ^[0-9]+$ ]] && [ "${#port}" -le 5 ] && [ "$((10#$port))" -ge 1 ] && [ "$((10#$port))" -le 65535 ] || {
    printf 'Invalid test port: %s\n' "$port" >&2
    exit 1
  }
done
export ARCHESTRA_BASE_URL="http://localhost:${BACKEND_PORT}"
export ARCHESTRA_FRONTEND_URL="http://localhost:${FRONTEND_PORT}"
export ARCHESTRA_NAMESPACE="${ARCHESTRA_NAMESPACE:-default}"
# One model URL for the seeded fixture and --print-env; CI seeds the same /v1 URL.
LLM_BASE_URL="http://ollama-mock.${ARCHESTRA_NAMESPACE}.svc.cluster.local:11434/v1"

log() { printf '%s\n' "$*" >&2; }

# resolve_version reads ARCHESTRA_VERSION from platform-version.txt, the
# release CI tests, so local tracks CI without re-hardcoding a tag here. Only
# called on the `up` path — --down and --print-env don't need it.
resolve_version() {
  if [ -n "${ARCHESTRA_VERSION:-}" ]; then
    return
  fi
  ARCHESTRA_VERSION=$("${REPO_ROOT}/scripts/platform-version.sh")
  ARCHESTRA_VERSION=${ARCHESTRA_VERSION#ARCHESTRA_VERSION=}
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
export ARCHESTRA_BASE_URL=${ARCHESTRA_BASE_URL}
export ARCHESTRA_FRONTEND_URL=${ARCHESTRA_FRONTEND_URL}
export ARCHESTRA_API_KEY=\$(${SCRIPT_DIR}/bootstrap-api-key.sh)
export ARCHESTRA_READONLY_VAULT_ENABLED=true
export ARCHESTRA_ENTERPRISE_LICENSE_ACTIVATED=true
export ARCHESTRA_AGENT_RUNTIME_ENABLED=true
export ARCHESTRA_TEST_IDP_ID=\$(ARCHESTRA_API_KEY=\$ARCHESTRA_API_KEY ${SCRIPT_DIR}/bootstrap-test-idp.sh)
export ARCHESTRA_TEST_LLM_BASE_URL=${LLM_BASE_URL}
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

# Keep every Kubernetes command, including child bootstrappers, on a private
# kubeconfig. Never rewrite the user's current context.
stack_tmp=$(mktemp -d)
trap 'rm -rf "$stack_tmp"' EXIT
export KUBECONFIG="${stack_tmp}/kubeconfig"
sed -e "s/hostPort: 9000/hostPort: ${BACKEND_PORT}/" \
    -e "s/hostPort: 3000/hostPort: ${FRONTEND_PORT}/" \
    -e "s/hostPort: 9050/hostPort: ${METRICS_PORT}/" \
    "$KIND_CONFIG" > "${stack_tmp}/kind.yaml"

# 1. Kind cluster — reuse only the explicitly selected test cluster.
if ! kind get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER"; then
  log "Creating kind cluster ${KIND_CLUSTER}..."
  kind create cluster --config "${stack_tmp}/kind.yaml" --name "$KIND_CLUSTER"
else
  log "Kind cluster ${KIND_CLUSTER} already exists, reusing."
fi

kind get kubeconfig --name "$KIND_CLUSTER" > "$KUBECONFIG"

# 2. Reuse a local image when available; fresh CI nodes pull once into containerd.
PLATFORM_IMAGE="archestra/platform:${ARCHESTRA_VERSION}"
if docker image inspect "$PLATFORM_IMAGE" >/dev/null 2>&1 && \
  kind load docker-image "$PLATFORM_IMAGE" --name "$KIND_CLUSTER" >/dev/null; then
  log "Loaded the cached ${PLATFORM_IMAGE} into kind."
else
  log "Pulling ${PLATFORM_IMAGE} directly inside the test nodes..."
  for node in $(kind get nodes --name "$KIND_CLUSTER"); do
    docker exec "$node" ctr --namespace=k8s.io images pull "docker.io/${PLATFORM_IMAGE}" >/dev/null
  done
fi

# 3. Helm install/upgrade the platform chart with the same values CI uses.
log "Helm installing ${HELM_RELEASE} (chart ${ARCHESTRA_VERSION})..."
helm upgrade --install "$HELM_RELEASE" "$HELM_CHART_OCI" \
  --version "$ARCHESTRA_VERSION" \
  --namespace "$ARCHESTRA_NAMESPACE" --create-namespace \
  --values "$HELM_VALUES" \
  --set "archestra.image=${PLATFORM_IMAGE}" \
  --set "archestra.env.ARCHESTRA_FRONTEND_URL=http://localhost:${FRONTEND_PORT}" \
  --atomic --timeout=5m >/dev/null

# 4. Wait for platform pods to be ready before deploying the dependencies
#    that the platform's healthcheck doesn't gate on.
log "Waiting for platform pods..."
kubectl wait --namespace "$ARCHESTRA_NAMESPACE" --for=condition=Ready pods -l app.kubernetes.io/name=archestra-platform --timeout=120s >/dev/null

# 5. Ollama mock + dev Vault — same manifests CI applies post-platform.
"${SCRIPT_DIR}/bootstrap-ollama-mock.sh"
"${SCRIPT_DIR}/bootstrap-vault.sh"

# 6. Backend health gate on the selected host port.
log "Waiting for backend at ${ARCHESTRA_BASE_URL}..."
deadline=$(($(date +%s) + 120))
while :; do
  c=$(curl -sS --connect-timeout 2 --max-time 3 -o /dev/null -w '%{http_code}' \
        "${ARCHESTRA_BASE_URL}/api/health" 2>/dev/null || echo "")
  case "$c" in 200|401) break ;; esac
  [ "$(date +%s)" -lt "$deadline" ] || { log "backend never became reachable"; exit 1; }
  sleep 2
done

# Agent Runtime acceptance requires the same real controller CI installs.
"${SCRIPT_DIR}/bootstrap-agent-sandbox.sh"

# 7. Seed an Ollama provider key so the LLM model tests find at least one model.
api_key=$("${SCRIPT_DIR}/bootstrap-api-key.sh")
ARCHESTRA_API_KEY="$api_key" ARCHESTRA_READONLY_VAULT_ENABLED=true \
  ARCHESTRA_TEST_LLM_BASE_URL="$LLM_BASE_URL" python3 "${SCRIPT_DIR}/bootstrap-test-llm-key.py" >/dev/null

# 8. Provision the EMC test IdP.
ARCHESTRA_API_KEY="$api_key" "${SCRIPT_DIR}/bootstrap-test-idp.sh" >/dev/null

log "Stack ready."
log
log "  eval \"\$(KIND_CLUSTER=${KIND_CLUSTER} ARCHESTRA_TEST_BACKEND_PORT=${BACKEND_PORT} ARCHESTRA_TEST_FRONTEND_PORT=${FRONTEND_PORT} scripts/bootstrap-local-stack.sh --print-env)\" && make testacc"
log
log "Tear down with:  KIND_CLUSTER=${KIND_CLUSTER} scripts/bootstrap-local-stack.sh --down"
