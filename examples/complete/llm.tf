resource "archestra_llm_provider_api_key" "ollama_vault" {
  name              = "Local Ollama"
  llm_provider      = "ollama"
  vault_secret_path = "secret/data/test/ollama"
  vault_secret_key  = "api_key"
  scope             = "org"
}

resource "archestra_llm_model" "llama3" {
  model_id                        = "llama3"
  custom_price_per_million_input  = "0.50"
  custom_price_per_million_output = "1.00"
  ignored                         = true # hide from UI picker; agents that reference llama3 by id still work
}

resource "archestra_llm_proxy" "shared_ollama" {
  name                 = "shared-ollama"
  description          = "Org-wide Ollama proxy fronted by Archestra (JWT-authenticated)"
  llm_model            = "llama3"
  llm_api_key_id       = archestra_llm_provider_api_key.ollama_vault.id
  identity_provider_id = archestra_identity_provider.oidc.id
  is_default           = true
}

resource "archestra_virtual_api_key" "shared_ollama_cicd" {
  llm_provider_api_key_id = archestra_llm_provider_api_key.ollama_vault.id
  name                    = "demo-cicd-virtual-key"
  scope                   = "org"
}

output "cicd_virtual_key" {
  description = "One-shot virtual API key — returned only on Create; stored sensitively in state."
  value       = archestra_virtual_api_key.shared_ollama_cicd.value
  sensitive   = true
}
