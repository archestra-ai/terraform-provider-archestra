#!/usr/bin/env bash
#
# Brings up the full Archestra dev stack as Docker containers (NOT k8s/Tilt)
# pinned to ARCHESTRA_VERSION. Result: every testacc — including the BYOS
# vault, EMC, and SSO subsets — passes locally without any TF_ACC gating.
#
# This is the one-shot reproduction of the manual setup that took ~30
# minutes the first time around. Re-run is idempotent (containers are
# stopped/recreated, the vault secret is re-seeded).
#
# Required tools: docker, curl, jq, kubectl IS NOT NEEDED.
#
# After it exits successfully:
#   eval "$(scripts/bootstrap-local-stack.sh --print-env)"
#   make testacc
#
# Print the env-var snippet without touching containers:
#   scripts/bootstrap-local-stack.sh --print-env
#
# Tear it all down:
#   scripts/bootstrap-local-stack.sh --down

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

NETWORK="${NETWORK:-archestra-net}"
VAULT_TOKEN="${VAULT_TOKEN:-root}"

# resolve_version reads ARCHESTRA_VERSION from CI's source of truth so local
# tracks CI without re-hardcoding a tag here. Only called on the `up` path —
# --down and --print-env don't need it, and shouldn't fail if the workflow
# file is moved.
resolve_version() {
  if [ -n "${ARCHESTRA_VERSION:-}" ]; then
    return
  fi
  ARCHESTRA_VERSION=$(grep -E '^\s*ARCHESTRA_VERSION:\s*"' \
    "${REPO_ROOT}/.github/workflows/on-pull-request.yml" 2>/dev/null \
    | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$ARCHESTRA_VERSION" ]; then
    echo "Could not resolve ARCHESTRA_VERSION (set it explicitly or fix .github/workflows/on-pull-request.yml)" >&2
    exit 1
  fi
}

log() { printf '%s\n' "$*" >&2; }

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
  log "Stopping containers..."
  docker rm -f archestra-platform archestra-vault ollama-mock 2>/dev/null || true
  docker network rm "$NETWORK" 2>/dev/null || true
  log "Down."
}

case "${1:-}" in
  --print-env) print_env; exit 0 ;;
  --down) down; exit 0 ;;
esac

resolve_version
log "Bringing up Archestra ${ARCHESTRA_VERSION} local stack..."

# 1. Shared docker network so platform/mock/vault can resolve each other by name.
docker network inspect "$NETWORK" >/dev/null 2>&1 || docker network create "$NETWORK" >/dev/null

# 2. Ollama mock — backend's testProviderApiKey hits /v1/models on Ollama.
#    hashicorp/http-echo serves a static OpenAI-shaped model list.
docker rm -f ollama-mock >/dev/null 2>&1 || true
docker run -d --name ollama-mock --network "$NETWORK" \
  hashicorp/http-echo:1.0.0 \
  -listen=:8080 \
  -text='{"object":"list","data":[{"id":"llama3","object":"model","created":1700000000,"owned_by":"local-stub"}]}' >/dev/null

# 3. Vault dev — required for ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT mode.
docker rm -f archestra-vault >/dev/null 2>&1 || true
docker run -d --name archestra-vault --network "$NETWORK" \
  --cap-add IPC_LOCK \
  -e VAULT_DEV_ROOT_TOKEN_ID="$VAULT_TOKEN" \
  -e VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200 \
  hashicorp/vault:1.18 >/dev/null

# Wait for vault then seed the test secret used by chat_llm_provider_api_key
# and catalog_item WithVaultRefs tests.
log "Waiting for vault..."
deadline=$(($(date +%s) + 30))
until docker exec -e VAULT_TOKEN="$VAULT_TOKEN" -e VAULT_ADDR=http://127.0.0.1:8200 \
        archestra-vault vault status >/dev/null 2>&1; do
  [ "$(date +%s)" -lt "$deadline" ] || { log "vault never came up"; exit 1; }
  sleep 1
done
docker exec -e VAULT_TOKEN="$VAULT_TOKEN" -e VAULT_ADDR=http://127.0.0.1:8200 \
  archestra-vault vault kv put secret/test/ollama api_key=test-api-key-value >/dev/null

# 4. Platform — EE license toggled on, secrets manager pointed at the dev vault,
#    Ollama base URL pointed at the mock so testProviderApiKey passes without
#    a real Ollama. Volumes pin postgres + app data so re-runs are idempotent.
docker rm -f archestra-platform >/dev/null 2>&1 || true
docker run -d --name archestra-platform --network "$NETWORK" \
  -p 9000:9000 -p 3000:3000 \
  -e ARCHESTRA_QUICKSTART=true \
  -e ARCHESTRA_ENTERPRISE_LICENSE_ACTIVATED=true \
  -e ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT \
  -e ARCHESTRA_HASHICORP_VAULT_ADDR=http://archestra-vault:8200 \
  -e ARCHESTRA_HASHICORP_VAULT_TOKEN="$VAULT_TOKEN" \
  -e ARCHESTRA_OLLAMA_BASE_URL=http://ollama-mock:8080/v1 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v archestra-postgres-data:/var/lib/postgresql/data \
  -v archestra-app-data:/app/data \
  "archestra/platform:${ARCHESTRA_VERSION}" >/dev/null

# 5. Health-gate the platform. /api/health 200/401 means the API is serving;
#    docker's health=starting is its own slower probe, ignore it.
log "Waiting for platform..."
deadline=$(($(date +%s) + 120))
while :; do
  c=$(curl -sS --connect-timeout 2 --max-time 3 -o /dev/null -w '%{http_code}' \
        http://localhost:9000/api/health 2>/dev/null || echo "")
  case "$c" in 200|401) break ;; esac
  [ "$(date +%s)" -lt "$deadline" ] || { log "platform never came up"; exit 1; }
  sleep 2
done

# 6. Seed an Ollama provider key so the LLM model tests find at least one model.
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

# 7. Provision the EMC test IdP.
ARCHESTRA_API_KEY="$api_key" "${SCRIPT_DIR}/bootstrap-test-idp.sh" >/dev/null

log "Stack ready."
log
log "  eval \"\$(scripts/bootstrap-local-stack.sh --print-env)\" && make testacc"
log
log "Tear down with:  scripts/bootstrap-local-stack.sh --down"
