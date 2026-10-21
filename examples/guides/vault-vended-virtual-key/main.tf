# Vend an Archestra virtual key through Terraform and store it in a team's
# Vault path. Archestra only reads from Vault; the HashiCorp Vault provider
# performs the write with the Vault token configured for Terraform.

terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
    }
    vault = {
      source = "hashicorp/vault"
    }
  }
}

provider "archestra" {}

# Uses VAULT_ADDR and VAULT_TOKEN from the environment.
provider "vault" {}

variable "team_name" {
  type    = string
  default = "engineering"
}

resource "archestra_team" "this" {
  name = var.team_name
}

# The upstream provider credential stays in Vault. Archestra reads it from
# secret/data/llm/openai at request time and never writes to that path.
resource "archestra_model_provider" "openai" {
  name              = "OpenAI (Vault)"
  llm_provider      = "openai"
  vault_secret_path = "secret/data/llm/openai"
  vault_secret_key  = "api_key"

  # Organization-owned. A service account cannot own a personal provider key,
  # so this is required when Terraform authenticates as one.
  shared = true
}

resource "archestra_virtual_key" "team" {
  name = "${var.team_name}-model-access"
  provider_api_keys = [{
    provider            = "openai"
    provider_api_key_id = archestra_model_provider.openai.id
  }]
  billing_team_id = archestra_team.this.id
  spend_cap       = { limit_value = 500, cleanup_interval = "calendar_month" }
  expires_at      = "2027-12-31T23:59:59Z"
  rotation_nonce  = "1"

  lifecycle {
    # Keeps a valid key available while rotation_nonce replaces it.
    create_before_destroy = true
  }
}

# Terraform state also holds this value, as a sensitive attribute.
resource "vault_kv_secret_v2" "team_virtual_key" {
  mount = "secret"
  name  = "teams/${var.team_name}/archestra-virtual-key"
  data_json = jsonencode({
    value       = archestra_virtual_key.team.value
    token_start = archestra_virtual_key.team.token_start
  })
}
