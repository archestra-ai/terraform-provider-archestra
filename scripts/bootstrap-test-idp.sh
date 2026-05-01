#!/usr/bin/env bash
#
# Provisions a throwaway OIDC identity provider so the EMC (enterprise managed
# config) acceptance tests can exercise the full round-trip. Prints ONLY the
# resulting IdP id on stdout; all other output goes to stderr:
#
#     export ARCHESTRA_TEST_IDP_ID=$(scripts/bootstrap-test-idp.sh)
#
# Required tools: curl, jq.
#
# Environment overrides:
#   ARCHESTRA_BASE_URL          Backend URL (default: http://localhost:9000)
#   ARCHESTRA_API_KEY           API key (required; pair with bootstrap-api-key.sh)
#   ARCHESTRA_TEST_IDP_PROVIDER  IdP providerId (default: ci-test-emc)
#   ARCHESTRA_TEST_IDP_DOMAIN    IdP domain     (default: ci.emc.test)

set -euo pipefail

BASE_URL="${ARCHESTRA_BASE_URL:-http://localhost:9000}"
API_KEY="${ARCHESTRA_API_KEY:-}"
PROVIDER_ID="${ARCHESTRA_TEST_IDP_PROVIDER:-ci-test-emc}"
DOMAIN="${ARCHESTRA_TEST_IDP_DOMAIN:-ci.emc.test}"

if [ -z "$API_KEY" ]; then
  echo "ARCHESTRA_API_KEY is required (run scripts/bootstrap-api-key.sh first)." >&2
  exit 1
fi

# Fail fast if the backend isn't reachable AND if the response isn't a sane
# health code (200 or 401). Catches the case where curl connects but the
# backend is in a 5xx state, which would otherwise produce a misleading
# "Failed to create identity provider" message later.
status=$(curl -sS --connect-timeout 3 --max-time 5 -o /dev/null \
  -w '%{http_code}' "${BASE_URL}/api/health" 2>/dev/null || echo "")
if [ -z "$status" ] || [ "$status" = "000" ]; then
  echo "Backend unreachable at ${BASE_URL}. Is the Archestra platform running?" >&2
  exit 1
fi
if [ "$status" != "200" ] && [ "$status" != "401" ]; then
  echo "Backend at ${BASE_URL} returned HTTP ${status} for /api/health; expected 200 or 401." >&2
  exit 1
fi

# Reuse an existing IdP with the same providerId if one is already present —
# the EE endpoint rejects duplicates, and re-running the bootstrap (locally or
# in re-runs of CI on a sticky cluster) should not be fatal.
existing=$(curl -sS --connect-timeout 5 --max-time 15 \
  -H "Authorization: ${API_KEY}" "${BASE_URL}/api/identity-providers" \
  | jq -r --arg providerId "$PROVIDER_ID" \
      'if type == "array" then . else (.data // []) end
       | map(select(.providerId == $providerId))
       | (.[0].id // empty)')

if [ -n "$existing" ]; then
  echo "Reusing existing IdP (providerId=${PROVIDER_ID}, id=${existing})." >&2
  printf '%s\n' "$existing"
  exit 0
fi

echo "Creating throwaway OIDC IdP (providerId=${PROVIDER_ID}) at ${BASE_URL}..." >&2
response=$(curl -sS --connect-timeout 5 --max-time 15 \
  -X POST "${BASE_URL}/api/identity-providers" \
  -H "Authorization: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d "$(jq -n --arg providerId "$PROVIDER_ID" --arg domain "$DOMAIN" '{
    providerId: $providerId,
    domain: $domain,
    issuer: "https://idp.ci.example.com",
    oidcConfig: {
      issuer: "https://idp.ci.example.com",
      discoveryEndpoint: "https://idp.ci.example.com/.well-known/openid-configuration",
      clientId: "ci-test",
      clientSecret: "ci-secret",
      pkce: false,
      skipDiscovery: true
    }
  }')")

idp_id=$(echo "$response" | jq -r '.id')
if [ -z "$idp_id" ] || [ "$idp_id" = "null" ]; then
  echo "Failed to create identity provider. Response: ${response}" >&2
  exit 1
fi

echo "IdP provisioned: ${idp_id}" >&2
printf '%s\n' "$idp_id"
