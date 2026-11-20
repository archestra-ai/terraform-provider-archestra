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

# A key for one team: it calls a model pool by name, and the team can use it.
resource "archestra_virtual_key" "team" {
  name           = "platform-team"
  model_pool_ids = [archestra_model_pool.example.id]
  permissions = [
    { team_id = archestra_team.platform.id, actions = ["read", "use"] },
  ]
}
