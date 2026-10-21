resource "archestra_virtual_key" "example" {
  name = "model-router"
  provider_api_keys = [{
    provider            = "openai"
    provider_api_key_id = archestra_model_provider.example.id
  }]
  spend_cap = { limit_value = 100, cleanup_interval = "calendar_month" }
  # Changing this value replaces the key and revokes its previous credential.
  rotation_nonce = "1"

  lifecycle {
    # Keeps a valid key available during rotation_nonce rotation.
    create_before_destroy = true
  }
}
