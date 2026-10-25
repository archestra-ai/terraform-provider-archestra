# --- Variables this file references ---
# Add to your variables.tf (set values via TF_VAR_<name> env vars or a
# gitignored terraform.tfvars):
#
#   variable "openai_api_key"                  { type = string, sensitive = true }
#   variable "anthropic_api_key_engineering"   { type = string, sensitive = true }
#   variable "azure_openai_key"                { type = string, sensitive = true }
#
# Also assumed declared elsewhere in your module: archestra_team.engineering.

# The plaintext key is sent to the backend
# on create and stored encrypted; pass via a TF variable so it never lands
# in version control.
resource "archestra_model_provider" "inline" {
  name         = "Production OpenAI Key"
  api_key      = var.openai_api_key
  llm_provider = "openai"
  is_primary   = true
}

# Additional provider key. Manage object access through direct grants.
resource "archestra_model_provider" "engineering_anthropic" {
  name         = "Engineering Anthropic Key"
  api_key      = var.anthropic_api_key_engineering
  llm_provider = "anthropic"
}

# Self-hosted Ollama via custom base URL.
# `api_key` can be any non-empty string; the backend doesn't use it for
# Ollama, but the wire schema requires the field.
resource "archestra_model_provider" "ollama_local" {
  name         = "Local Ollama"
  api_key      = "unused"
  llm_provider = "ollama"
  base_url     = "http://ollama.internal.acme.com:11434"
}

# Azure OpenAI. Set the deployment endpoint via base_url.
resource "archestra_model_provider" "azure_openai" {
  name         = "Azure OpenAI East-US"
  api_key      = var.azure_openai_key
  llm_provider = "azure"
  base_url     = "https://acme-openai-eastus.openai.azure.com/"
}
