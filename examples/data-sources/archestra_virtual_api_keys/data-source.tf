# List every virtual API key visible to the caller (paginated
# exhaustively). The full token `value` is never returned — use the
# `archestra_virtual_api_key` resource for create-time issuance.

data "archestra_virtual_api_keys" "engineering" {
  llm_provider_api_key_id = archestra_llm_provider_api_key.shared.id
  search                  = "engineering"
}

output "engineering_virtual_key_ids" {
  value = [for k in data.archestra_virtual_api_keys.engineering.virtual_api_keys : k.id]
}
