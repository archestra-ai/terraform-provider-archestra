#!/usr/bin/env bash
#
# Adds Crossplane and an out-of-cluster provider-archestra controller to the
# Kind stack created by scripts/bootstrap-local-stack.sh, for both-scope
# lifecycle probes (scripts/integration/crossplane-lifecycle.py).
#
#   1. Installs Crossplane ${CROSSPLANE_VERSION} (Helm) into the Kind cluster.
#   2. Builds the Terraform provider, generates both Crossplane scopes and
#      builds the controller (skip with CROSSPLANE_SKIP_BUILD=1).
#   3. Applies the generated CRDs for both scopes.
#   4. Builds a credentials Secret from `bootstrap-local-stack.sh --print-env`
#      and creates the cluster ProviderConfig plus the namespaced
#      ClusterProviderConfig and ProviderConfig, all named ${PROVIDER_CONFIG}.
#   5. Deploys the remote MCP fixture (scripts/k8s/provider-mcp-mock.yaml).
#   6. Starts the controller on the host with Terraform 1.5.7 and the locally
#      built provider served from a filesystem mirror.
#
# Required tools: kind, kubectl, helm, curl, go, goimports, make, python3.
#
# Re-run is idempotent and restarts the controller with freshly built binaries.
# Use the same KIND_CLUSTER / ARCHESTRA_TEST_*_PORT values as the stack.
#
#   scripts/bootstrap-crossplane.sh          # bring up
#   scripts/bootstrap-crossplane.sh --down   # stop the controller, remove what this script added
#
# Every Kubernetes command uses a private kubeconfig under the work directory;
# the user's current context is never changed. The API key is written only to
# the work directory (mode 0600) and the in-cluster Secret.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

KIND_CLUSTER="${KIND_CLUSTER:-archestra-ci-cluster}"
CROSSPLANE_VERSION="${CROSSPLANE_VERSION:-2.2.1}"
CROSSPLANE_NAMESPACE="${CROSSPLANE_NAMESPACE:-crossplane-system}"
PROVIDER_CONFIG="${PROVIDER_CONFIG:-default}"
TERRAFORM_VERSION=1.5.7
# Version label of the locally built provider inside the filesystem mirror.
TF_PROVIDER_VERSION="${CROSSPLANE_TF_PROVIDER_VERSION:-3.0.0}"
# Lifecycle probes repair out-of-band drift within 75 seconds.
POLL_INTERVAL="${CROSSPLANE_POLL_INTERVAL:-10s}"
METRICS_ADDRESS="${CROSSPLANE_METRICS_ADDRESS:-127.0.0.1:18080}"
WORK_DIR="${CROSSPLANE_WORK_DIR:-${HOME}/.cache/archestra-provider-crossplane/${KIND_CLUSTER}}"

GOOS_ARCH="$(go env GOOS)_$(go env GOARCH)"
PROVIDER_BIN="${REPO_ROOT}/terraform-provider-archestra"
CONTROLLER_BIN="${REPO_ROOT}/crossplane/_output/bin/${GOOS_ARCH}/provider"
MIRROR_DIR="${WORK_DIR}/provider-mirror"
PLUGIN_DIR="${MIRROR_DIR}/registry.terraform.io/archestra-ai/archestra/${TF_PROVIDER_VERSION}/${GOOS_ARCH}"
TERRAFORM_DIR="${WORK_DIR}/terraform-${TERRAFORM_VERSION}"
PID_FILE="${WORK_DIR}/controller.pid"

log() { printf '%s\n' "$*" >&2; }

require_tool() {
  command -v "$1" >/dev/null 2>&1 || { log "Missing required tool: $1"; exit 1; }
}

stop_controller() {
  if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    log "Stopping controller (pid $(cat "$PID_FILE"))..."
    kill "$(cat "$PID_FILE")"
    for _ in $(seq 1 30); do
      kill -0 "$(cat "$PID_FILE")" 2>/dev/null || break
      sleep 1
    done
  fi
  rm -f "$PID_FILE"
}

use_private_kubeconfig() {
  mkdir -p "$WORK_DIR"
  chmod 700 "$WORK_DIR"
  kind get kubeconfig --name "$KIND_CLUSTER" > "${WORK_DIR}/kubeconfig"
  export KUBECONFIG="${WORK_DIR}/kubeconfig"
}

down() {
  stop_controller
  if ! kind get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER"; then
    log "Kind cluster ${KIND_CLUSTER} not found; nothing else to remove."
    return
  fi
  use_private_kubeconfig
  kubectl delete -n default -f "${SCRIPT_DIR}/k8s/provider-mcp-mock.yaml" --ignore-not-found
  kubectl delete -n default configmap provider-mcp-mock-script --ignore-not-found
  kubectl delete providerconfigs.archestra.crossplane.io "$PROVIDER_CONFIG" --ignore-not-found 2>/dev/null || true
  kubectl delete clusterproviderconfigs.archestra.m.crossplane.io "$PROVIDER_CONFIG" --ignore-not-found 2>/dev/null || true
  kubectl delete -n "$CROSSPLANE_NAMESPACE" providerconfigs.archestra.m.crossplane.io "$PROVIDER_CONFIG" --ignore-not-found 2>/dev/null || true
  if [ -d "${REPO_ROOT}/crossplane/package/crds" ]; then
    kubectl delete -f "${REPO_ROOT}/crossplane/package/crds" --ignore-not-found >/dev/null
  fi
  helm uninstall crossplane --namespace "$CROSSPLANE_NAMESPACE" --wait 2>/dev/null || true
  kubectl delete namespace "$CROSSPLANE_NAMESPACE" --ignore-not-found
  log "Removed Crossplane, the provider CRDs and fixtures from ${KIND_CLUSTER}. Work directory kept: ${WORK_DIR}"
}

case "${1:-}" in
  --down) down; exit 0 ;;
  "") ;;
  *) log "Usage: $0 [--down]"; exit 2 ;;
esac

for tool in kind kubectl helm curl go goimports make python3; do require_tool "$tool"; done
kind get clusters 2>/dev/null | grep -qx "$KIND_CLUSTER" || {
  log "Kind cluster ${KIND_CLUSTER} not found. Run scripts/bootstrap-local-stack.sh first."
  exit 1
}
use_private_kubeconfig

# 1. Crossplane.
log "Installing Crossplane ${CROSSPLANE_VERSION}..."
# Run from the work directory: Helm prefers a local ./crossplane directory over the repository chart.
(cd "$WORK_DIR" && helm upgrade --install crossplane crossplane \
  --repo https://charts.crossplane.io/stable --version "$CROSSPLANE_VERSION" \
  --namespace "$CROSSPLANE_NAMESPACE" --create-namespace --wait --timeout=5m >/dev/null)

# 2. Provider, generated scopes and controller.
if [ "${CROSSPLANE_SKIP_BUILD:-0}" != 1 ]; then
  log "Building the Terraform provider and both Crossplane scopes..."
  (cd "$REPO_ROOT" && go build -o terraform-provider-archestra .)
  make -C "${REPO_ROOT}/crossplane" submodules generate go.build >&2
fi
for binary in "$PROVIDER_BIN" "$CONTROLLER_BIN"; do
  [ -x "$binary" ] || { log "Missing build output: $binary"; exit 1; }
done

# 3. CRDs for both scopes.
log "Applying generated CRDs..."
kubectl apply --server-side --force-conflicts -f "${REPO_ROOT}/crossplane/package/crds" >/dev/null
kubectl wait --for=condition=Established crd --all --timeout=120s >/dev/null

# 4. Credentials and ProviderConfigs.
log "Creating credentials and ProviderConfigs..."
(
  eval "$(KIND_CLUSTER="$KIND_CLUSTER" "${SCRIPT_DIR}/bootstrap-local-stack.sh" --print-env)"
  umask 077
  printf '%s' "$ARCHESTRA_API_KEY" > "${WORK_DIR}/api-key"
  python3 -c 'import json, os; print(json.dumps({"api_key": os.environ["ARCHESTRA_API_KEY"], "base_url": os.environ["ARCHESTRA_BASE_URL"]}))' \
    > "${WORK_DIR}/credentials.json"
)
kubectl create secret generic archestra-creds -n "$CROSSPLANE_NAMESPACE" \
  --from-file=credentials="${WORK_DIR}/credentials.json" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
rm -f "${WORK_DIR}/credentials.json"
kubectl apply -f - >/dev/null <<EOF
apiVersion: archestra.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: ${PROVIDER_CONFIG}
spec:
  credentials:
    source: Secret
    secretRef: {namespace: ${CROSSPLANE_NAMESPACE}, name: archestra-creds, key: credentials}
---
apiVersion: archestra.m.crossplane.io/v1beta1
kind: ClusterProviderConfig
metadata:
  name: ${PROVIDER_CONFIG}
spec:
  credentials:
    source: Secret
    secretRef: {namespace: ${CROSSPLANE_NAMESPACE}, name: archestra-creds, key: credentials}
---
apiVersion: archestra.m.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: ${PROVIDER_CONFIG}
  namespace: ${CROSSPLANE_NAMESPACE}
spec:
  credentials:
    source: Secret
    secretRef: {namespace: ${CROSSPLANE_NAMESPACE}, name: archestra-creds, key: credentials}
EOF

# 5. Remote MCP fixture reachable from the backend pod.
log "Deploying the MCP fixture..."
kubectl create configmap provider-mcp-mock-script -n default \
  --from-file=mcp-mock.py="${SCRIPT_DIR}/integration/mcp-mock.py" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl apply -f "${SCRIPT_DIR}/k8s/provider-mcp-mock.yaml" >/dev/null
kubectl rollout restart deployment/provider-mcp-mock -n default >/dev/null
kubectl rollout status deployment/provider-mcp-mock -n default --timeout=120s >/dev/null

# 6. Terraform 1.5.7, the provider mirror and the controller.
if [ ! -x "${TERRAFORM_DIR}/terraform" ]; then
  log "Downloading Terraform ${TERRAFORM_VERSION}..."
  mkdir -p "$TERRAFORM_DIR"
  curl -fsSL "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_${GOOS_ARCH}.zip" \
    -o "${TERRAFORM_DIR}/terraform.zip"
  python3 -c 'import sys, zipfile; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])' "${TERRAFORM_DIR}/terraform.zip" "$TERRAFORM_DIR"
  chmod +x "${TERRAFORM_DIR}/terraform"
  rm -f "${TERRAFORM_DIR}/terraform.zip"
fi
rm -rf "$MIRROR_DIR"
mkdir -p "$PLUGIN_DIR"
cp "$PROVIDER_BIN" "${PLUGIN_DIR}/terraform-provider-archestra_v${TF_PROVIDER_VERSION}"
cat > "${WORK_DIR}/terraformrc" <<EOF
provider_installation {
  filesystem_mirror {
    path    = "${MIRROR_DIR}"
    include = ["*/*"]
  }
  direct {
    exclude = ["*/*"]
  }
}
EOF

stop_controller
log "Starting the controller..."
(
  cd "$WORK_DIR"
  PATH="${TERRAFORM_DIR}:${PATH}" TF_CLI_CONFIG_FILE="${WORK_DIR}/terraformrc" TF_FORK=0 \
    nohup "$CONTROLLER_BIN" --debug \
      --terraform-version="$TERRAFORM_VERSION" \
      --terraform-provider-source=archestra-ai/archestra \
      --terraform-provider-version="$TF_PROVIDER_VERSION" \
      --poll="$POLL_INTERVAL" \
      --metrics-bind-address="$METRICS_ADDRESS" \
      --certs-dir= \
      > "${WORK_DIR}/controller.log" 2>&1 &
  echo $! > "$PID_FILE"
)
sleep 5
kill -0 "$(cat "$PID_FILE")" 2>/dev/null || {
  log "Controller exited; see ${WORK_DIR}/controller.log"
  exit 1
}

log "Crossplane ready. Controller log: ${WORK_DIR}/controller.log"
log
log "  scripts/integration/crossplane-lifecycle.py --cases apps \\"
log "    --api-url \"\$ARCHESTRA_BASE_URL\" --key-file ${WORK_DIR}/api-key --readonly-vault \\"
log "    --llm-base-url \"\$ARCHESTRA_TEST_LLM_BASE_URL\" \\"
log "    --mcp-url http://provider-mcp-mock.default.svc.cluster.local:19006/mcp \\"
log "    --kubeconfig ${WORK_DIR}/kubeconfig --provider-config ${PROVIDER_CONFIG} \\"
log "    --terraform-binary ${PROVIDER_BIN} --controller-binary ${CONTROLLER_BIN} \\"
log "    --evidence crossplane-evidence.json"
log
log "Tear down with:  KIND_CLUSTER=${KIND_CLUSTER} scripts/bootstrap-crossplane.sh --down"
