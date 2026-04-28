# Inline API key (default DB-backed secrets mode).
resource "archestra_llm_provider_api_key" "inline" {
  name                    = "Production OpenAI Key"
  api_key                 = var.openai_api_key
  llm_provider            = "openai"
  is_organization_default = true
}

# Vault reference — required when the Archestra backend runs in BYOS
# (READONLY_VAULT) mode. The secret at the given path/key is read by the
# backend at creation time; the plaintext never touches Terraform state.
resource "archestra_llm_provider_api_key" "vault_backed" {
  name              = "Production Anthropic Key"
  llm_provider      = "anthropic"
  vault_secret_path = "secret/data/archestra/llm"
  vault_secret_key  = "anthropic_api_key"
  scope             = "org"
}
