resource "archestra_oauth_client" "example" {
  name       = "llm-application"
  scopes     = ["llm:proxy"]
  grant_type = "client_credentials"
  provider_api_keys = [{
    provider            = "openai"
    provider_api_key_id = archestra_llm_provider_api_key.example.id
  }]
  # Changing this value rotates the secret and preserves the client ID.
  rotation_nonce = "1"
}
