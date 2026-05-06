#!/usr/bin/env bash
#
# Bootstraps an API key against an Archestra backend by signing in with admin
# credentials (better-auth email flow) and creating a fresh better-auth API
# key. Prints ONLY the resulting key on stdout — all other output goes to
# stderr — so the caller can do:
#
#     export ARCHESTRA_API_KEY=$(scripts/bootstrap-api-key.sh)
#
# Used by both CI (.github/workflows/on-pull-request.yml) and local
# development against a backend with the default admin seed.
#
# Required tools: curl, jq, mktemp.
#
# Environment overrides:
#   ARCHESTRA_BASE_URL              Backend URL (default: http://localhost:9000)
#   ARCHESTRA_ADMIN_EMAIL           Admin email   (default: admin@example.com)
#   ARCHESTRA_ADMIN_PASSWORD        Admin password (default: password)
#   ARCHESTRA_API_KEY_NAME          API key label (default: terraform-acceptance-tests)
#   ARCHESTRA_API_KEY_EXPIRES_IN    TTL in seconds (default: 604800 = 7 days)

set -euo pipefail

BASE_URL="${ARCHESTRA_BASE_URL:-http://localhost:9000}"
ADMIN_EMAIL="${ARCHESTRA_ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ARCHESTRA_ADMIN_PASSWORD:-password}"
KEY_NAME="${ARCHESTRA_API_KEY_NAME:-terraform-acceptance-tests}"
EXPIRES_IN="${ARCHESTRA_API_KEY_EXPIRES_IN:-604800}"

cookies=$(mktemp)
trap 'rm -f "$cookies"' EXIT

# Fail fast if the backend isn't reachable. The /api/health endpoint returns
# 200 even when unauthenticated requests are otherwise rejected, so we use it
# as a liveness probe. --connect-timeout caps the TCP wait; --max-time caps
# the whole probe.
if ! curl -sS --connect-timeout 3 --max-time 5 -o /dev/null -w '%{http_code}' \
    "${BASE_URL}/api/health" >/tmp/.archestra-health.$$ 2>/dev/null; then
  rm -f /tmp/.archestra-health.$$
  echo "Backend unreachable at ${BASE_URL}. Is the Archestra platform running?" >&2
  exit 1
fi
status=$(cat /tmp/.archestra-health.$$)
rm -f /tmp/.archestra-health.$$
if [ "$status" != "200" ] && [ "$status" != "401" ]; then
  echo "Backend at ${BASE_URL} returned HTTP ${status} for /api/health; expected 200 or 401." >&2
  exit 1
fi

echo "Signing in as ${ADMIN_EMAIL} at ${BASE_URL}..." >&2
login_response=$(curl -sS --connect-timeout 5 --max-time 15 \
  -c "$cookies" -b "$cookies" \
  -X POST "${BASE_URL}/api/auth/sign-in/email" \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:3000" \
  -d "$(jq -n --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" \
    '{email:$email, password:$password}')")

if ! echo "$login_response" | jq -e '.user.id' >/dev/null 2>&1; then
  echo "Login failed. Response: ${login_response}" >&2
  exit 1
fi

echo "Creating API key '${KEY_NAME}' (expires in ${EXPIRES_IN}s)..." >&2
key_response=$(curl -sS --connect-timeout 5 --max-time 15 \
  -b "$cookies" \
  -X POST "${BASE_URL}/api/auth/api-key/create" \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:3000" \
  -d "$(jq -n --arg name "$KEY_NAME" --argjson expiresIn "$EXPIRES_IN" \
    '{name:$name, expiresIn:$expiresIn}')")

api_key=$(echo "$key_response" | jq -r '.key')
if [ -z "$api_key" ] || [ "$api_key" = "null" ]; then
  echo "Failed to create API key. Response: ${key_response}" >&2
  exit 1
fi

echo "API key created." >&2
printf '%s\n' "$api_key"
