resource "archestra_virtual_api_key" "example" {
  name = "model-router"
  provider_api_keys = [{
    provider            = "openai"
    provider_api_key_id = archestra_llm_provider_api_key.example.id
  }]
  spend_cap = { limit_value = 100, cleanup_interval = "calendar_month" }
  # Changing this value replaces the key and revokes its previous credential.
  rotation_nonce = "1"
}
