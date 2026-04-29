# Inline API key — DB-backed secrets mode. The plaintext key is sent to the
# backend on create and stored encrypted; pass via a TF variable so it never
# lands in version control.
resource "archestra_llm_provider_api_key" "inline" {
  name                    = "Production OpenAI Key"
  api_key                 = var.openai_api_key
  llm_provider            = "openai"
  is_organization_default = true
}

# Vault-backed key — required when the backend runs in BYOS
# (`ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT`) mode. The backend reads the
# secret at creation time; the plaintext never touches Terraform state.
resource "archestra_llm_provider_api_key" "vault_backed" {
  name              = "Production Anthropic Key"
  llm_provider      = "anthropic"
  vault_secret_path = "secret/data/archestra/llm"
  vault_secret_key  = "anthropic_api_key"
  scope             = "org"
}

# Team-scoped key — billable spend rolls up to the engineering team only.
resource "archestra_llm_provider_api_key" "engineering_anthropic" {
  name         = "Engineering Anthropic Key"
  api_key      = var.anthropic_api_key_engineering
  llm_provider = "anthropic"
  scope        = "team"
  team_id      = archestra_team.engineering.id
}

# Self-hosted Ollama via custom base URL — no API key required, but the
# resource still needs `api_key` set (the backend ignores its value for Ollama).
resource "archestra_llm_provider_api_key" "ollama_local" {
  name         = "Local Ollama"
  api_key      = "unused"
  llm_provider = "ollama"
  base_url     = "http://ollama.internal.acme.com:11434"
}

# Azure OpenAI — set the deployment endpoint via base_url.
resource "archestra_llm_provider_api_key" "azure_openai" {
  name         = "Azure OpenAI East-US"
  api_key      = var.azure_openai_key
  llm_provider = "azure"
  base_url     = "https://acme-openai-eastus.openai.azure.com/"
}
