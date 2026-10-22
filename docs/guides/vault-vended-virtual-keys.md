---
page_title: "Vault-Vended Virtual Keys - archestra Provider"
subcategory: ""
description: |-
  Create virtual keys with Terraform and store their values in team Vault paths.
---

# Vault-vended virtual keys

An `archestra_virtual_key` is a revocable bearer credential that routes model
requests through Archestra to one or more model providers. Terraform can create
one key per team and write each value to that team's Vault path, so workloads
read their key from Vault instead of from Terraform outputs.

Archestra never writes to your Vault. With Vault-backed secrets (BYOS), the
platform only reads the provider credential named by `vault_secret_path` and
`vault_secret_key`. The write in this guide is done by the HashiCorp Vault
provider, using the Vault token configured for Terraform.

## Example

```terraform
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
    value = archestra_virtual_key.team.value
  })
}
```

The example:

- references an upstream provider credential that already exists in Vault,
  as an organization-owned model provider (`shared = true`). Keep `shared`
  when Terraform authenticates as a service account: a service account cannot
  own a personal provider key;
- creates a virtual key routed to that provider, billed to the team, with a
  monthly spend cap and an absolute expiry;
- writes the key value to `secret/data/teams/<team>/archestra-virtual-key`.

## Rotation

Change `rotation_nonce` to replace the key. With
`create_before_destroy = true`, Terraform creates the new key, updates the
Vault secret and then revokes the old key, so a valid key exists throughout.

## Terraform state

The key value is stored in Terraform state as a sensitive attribute. Sensitive
values are hidden from plan output but are readable by anyone who can read the
state. Use an encrypted remote backend with restricted access.

## Import and recovery

Archestra returns a key value on creation. On import, or when a refresh finds
no value in state, the provider requests the value from the API. Only the
identity that created the key can reveal it. For any other identity the API
returns 403; the provider then leaves `value` null without an error, and the key's ID and settings are read
as usual. Import still succeeds; identify such a key by its `id`.

A user can recover keys they created on every supported release. A service
account can recover keys it created starting with Archestra 1.5.0-rc.1. Earlier
releases, including the 1.4 releases, return 403, so a service account that
loses its state there must rotate the key rather than recover its value.
