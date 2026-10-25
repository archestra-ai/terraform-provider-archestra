#!/usr/bin/env bash
#
# Deploys a dev-mode Vault and restarts the
# Archestra backend so it picks up the now-reachable Vault. Required for
# managed Vault acceptance tests.
#
# WARNING: Dev mode disables auth and persists nothing — only use against
# ephemeral test clusters. Logic is shared between CI and any Kind-based
# local cluster; .github/values-ci.yaml configures the backend with
# ARCHESTRA_HASHICORP_VAULT_ADDR + ARCHESTRA_HASHICORP_VAULT_TOKEN that
# match the dev server.
#
# Required tools: kubectl with a context pointing at the target cluster.
#
# Reapplying the dev manifest preserves the running test Vault.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="${SCRIPT_DIR}/k8s/vault-dev.yaml"

ARCHESTRA_NAMESPACE="${ARCHESTRA_NAMESPACE:-archestra}"
ARCHESTRA_SELECTOR="${ARCHESTRA_SELECTOR:-app.kubernetes.io/name=archestra-platform}"
case "${ARCHESTRA_SKIP_BACKEND_RESTART:-false}" in
  true|false) ;;
  *) echo "ARCHESTRA_SKIP_BACKEND_RESTART must be true or false" >&2; exit 1 ;;
esac

echo "Applying Vault dev manifest from ${MANIFEST}..." >&2
kubectl apply -f "$MANIFEST"

echo "Waiting for vault rollout..." >&2
kubectl rollout status deployment/vault --namespace default --timeout=120s

# The Archestra backend reads Vault config at startup; if it came up before
# Vault was reachable, restart so it can connect now.
if [ "${ARCHESTRA_SKIP_BACKEND_RESTART:-false}" != true ]; then
  echo "Restarting Archestra backend (namespace=${ARCHESTRA_NAMESPACE}, selector=${ARCHESTRA_SELECTOR})..." >&2
  kubectl rollout restart deployment --namespace "$ARCHESTRA_NAMESPACE" --selector="$ARCHESTRA_SELECTOR"
  kubectl rollout status deployment --namespace "$ARCHESTRA_NAMESPACE" --selector="$ARCHESTRA_SELECTOR" --timeout=180s
fi

echo "Vault ready at http://vault.default.svc.cluster.local:8200 (token=root)" >&2
