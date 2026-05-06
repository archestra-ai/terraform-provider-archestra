# --- Backend secrets mode matters ---
#
# The Archestra backend runs in one of two modes:
#
#   - DB mode (default): inline `api_key` is encrypted server-side and stored
#     in Postgres. `vault_secret_path` / `vault_secret_key` are rejected.
#   - BYOS / READONLY_VAULT mode: `vault_secret_path` + `vault_secret_key` are
#     required for any provider that needs a real key. Inline `api_key` is
#     rejected with "Either apiKey or both vaultSecretPath and vaultSecretKey
#     must be provided" on the wire — even for providers like Ollama that
#     don't actually use the key (the backend validates the SHAPE before
#     looking at the provider).
#
# Detect the mode by checking `ARCHESTRA_SECRETS_MANAGER` on the running
# backend. The README has a guide for activating BYOS mode and seeding Vault.
#
# --- Variables this file references ---
# Add to your variables.tf (set values via TF_VAR_<name> env vars or a
# gitignored terraform.tfvars):
#
#   variable "openai_api_key"                  { type = string, sensitive = true }
#   variable "anthropic_api_key_engineering"   { type = string, sensitive = true }
#   variable "azure_openai_key"                { type = string, sensitive = true }
#
# Also assumed declared elsewhere in your module: archestra_team.engineering.

# Inline API key — DB MODE ONLY. The plaintext key is sent to the backend
# on create and stored encrypted; pass via a TF variable so it never lands
# in version control.
resource "archestra_llm_provider_api_key" "inline" {
  name                    = "Production OpenAI Key"
  api_key                 = var.openai_api_key
  llm_provider            = "openai"
  is_organization_default = true
}

# Vault-backed key — REQUIRED in BYOS mode, accepted in either mode.
#
# `vault_secret_path` is illustrative — replace with a real KV v2 path you
# have seeded yourself. The backend reads `secret/data/<path>` and looks up
# the named key inside the resulting JSON object. Seed it before applying:
#
#   vault kv put secret/<your-org>/llm anthropic_api_key=sk-ant-...
#
# (Note: `vault kv put` rewrites the path to `secret/data/<your-org>/llm`
# under the hood for KV v2 — the path you give Terraform must include the
# `data/` segment.)
resource "archestra_llm_provider_api_key" "vault_backed" {
  name              = "Production Anthropic Key"
  llm_provider      = "anthropic"
  vault_secret_path = "secret/data/your-org/llm" # placeholder — seed your own
  vault_secret_key  = "anthropic_api_key"
  scope             = "org"
}

# Team-scoped key — DB MODE ONLY. Billable spend rolls up to the engineering
# team only. In BYOS mode, swap to `vault_secret_path` / `vault_secret_key`.
resource "archestra_llm_provider_api_key" "engineering_anthropic" {
  name         = "Engineering Anthropic Key"
  api_key      = var.anthropic_api_key_engineering
  llm_provider = "anthropic"
  scope        = "team"
  team_id      = archestra_team.engineering.id
}

# Self-hosted Ollama via custom base URL — DB MODE.
# `api_key` can be any non-empty string; the backend doesn't use it for
# Ollama, but the wire schema requires the field.
resource "archestra_llm_provider_api_key" "ollama_local_db" {
  name         = "Local Ollama"
  api_key      = "unused"
  llm_provider = "ollama"
  base_url     = "http://ollama.internal.acme.com:11434"
}

# Self-hosted Ollama — BYOS MODE.
# Even though Ollama doesn't need a real key, the backend still requires
# the vault_* form (it validates the wire shape before checking the
# provider). Seed a placeholder secret and reference it.
resource "archestra_llm_provider_api_key" "ollama_local_byos" {
  name              = "Local Ollama"
  llm_provider      = "ollama"
  base_url          = "http://ollama.internal.acme.com:11434"
  vault_secret_path = "secret/data/your-org/ollama"
  vault_secret_key  = "api_key"
}

# Azure OpenAI — DB MODE shown. Set the deployment endpoint via base_url.
resource "archestra_llm_provider_api_key" "azure_openai" {
  name         = "Azure OpenAI East-US"
  api_key      = var.azure_openai_key
  llm_provider = "azure"
  base_url     = "https://acme-openai-eastus.openai.azure.com/"
}
