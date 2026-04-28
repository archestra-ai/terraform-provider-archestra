#!/usr/bin/env bash
#
# Deploys a dev-mode Vault, seeds the test secret, and restarts the
# Archestra backend so it picks up the now-reachable Vault. Required for
# BYOS-mode acceptance tests (ARCHESTRA_READONLY_VAULT_ENABLED=true).
#
# WARNING: Dev mode disables auth and persists nothing — only use against
# ephemeral test clusters. Logic is shared between CI and any Kind-based
# local cluster; .github/values-ci.yaml configures the backend with
# ARCHESTRA_HASHICORP_VAULT_ADDR + ARCHESTRA_HASHICORP_VAULT_TOKEN that
# match the values seeded here.
#
# Required tools: kubectl with a context pointing at the target cluster.
#
# Idempotent: kubectl apply / Vault dev re-seed are both safe to re-run.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="${SCRIPT_DIR}/k8s/vault-dev.yaml"

ARCHESTRA_NAMESPACE="${ARCHESTRA_NAMESPACE:-archestra}"
ARCHESTRA_SELECTOR="${ARCHESTRA_SELECTOR:-app.kubernetes.io/name=archestra-platform}"

echo "Applying Vault dev manifest from ${MANIFEST}..." >&2
kubectl apply -f "$MANIFEST"

echo "Waiting for vault rollout..." >&2
kubectl rollout status deployment/vault --namespace default --timeout=120s

# Seed the secret referenced by LLM provider API key tests.
echo "Seeding secret/data/test/ollama..." >&2
kubectl run vault-seed --rm -i --restart=Never --image=curlimages/curl:8.10.1 -- \
  curl -sS --connect-timeout 5 --max-time 15 \
    -X POST "http://vault.default.svc.cluster.local:8200/v1/secret/data/test/ollama" \
    -H "X-Vault-Token: root" \
    -H "Content-Type: application/json" \
    -d '{"data":{"api_key":"test-api-key-value"}}'

# The Archestra backend reads Vault config at startup; if it came up before
# Vault was reachable, restart so it can connect now.
echo "Restarting Archestra backend (namespace=${ARCHESTRA_NAMESPACE}, selector=${ARCHESTRA_SELECTOR})..." >&2
kubectl rollout restart deployment --namespace "$ARCHESTRA_NAMESPACE" --selector="$ARCHESTRA_SELECTOR"
kubectl rollout status deployment --namespace "$ARCHESTRA_NAMESPACE" --selector="$ARCHESTRA_SELECTOR" --timeout=180s

echo "Vault ready at http://vault.default.svc.cluster.local:8200 (token=root)" >&2
