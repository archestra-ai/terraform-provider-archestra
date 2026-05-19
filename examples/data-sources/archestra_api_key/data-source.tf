# Lookup an existing api key by ID. The full `value` is never returned —
# archestra_api_key issues it once at Create time only. Use this for
# audit / rotation-status workflows.
data "archestra_api_key" "ci_bot" {
  id = "33333333-3333-4333-a333-333333333333"
}

output "ci_bot_enabled" {
  value = data.archestra_api_key.ci_bot.enabled
}

output "ci_bot_expires_at" {
  value = data.archestra_api_key.ci_bot.expires_at
}
