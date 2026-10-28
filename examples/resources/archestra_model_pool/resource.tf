# Requires Archestra 1.5.0-rc.12's credential-owned grant API and
# ARCHESTRA_LLM_MODEL_POOLS_ENABLED=true (or ARCHESTRA_BETA=true).
# Use a provider release containing this resource; pin it in required_providers
# and commit .terraform.lock.hcl. See the model-pools guide for setup and import.
# These are existing shared provider-key UUIDs, also available as
# archestra_model_provider.<name>.id. Personal keys cannot be pooled.
variable "primary_provider_key_id" {
  type = string
}
variable "secondary_provider_key_id" {
  type = string
}
variable "fallback_provider_key_id" {
  type = string
}


resource "archestra_model_pool" "balanced" {
  name = "gpt-4.1"
  members = [
    {
      provider_api_key_id = var.primary_provider_key_id
      model_id            = "gpt-4.1-primary"
      priority            = 1
      weight              = 3
      enabled             = true
    },
    {
      provider_api_key_id = var.secondary_provider_key_id
      model_id            = "gpt-4.1-secondary"
      priority            = 1
      weight              = 1
      enabled             = true
    },
    {
      provider_api_key_id = var.fallback_provider_key_id
      model_id            = "gpt-4.1"
      priority            = 2
      weight              = 1
      enabled             = true
    },
  ]
}

# The first two members split new selections 3:1 while healthy. Priority 2
# serves only after priority 1 fails. Sessions remain sticky while healthy.
# Provider type comes from each saved key; model_id is its upstream model or
# deployment identifier, not an archestra_model resource UUID.
# Virtual keys mapping a member key already have access. Manage additional
# pool grants through credential settings; virtual_api_key_ids is read-only.
# The entire member set is replaced on every update.
# Cooldowns, retry rules, timeouts and routing strategy are server-owned.
# This API does not add Responses API support or utilization-aware routing.

# Adopt a pool created in the UI before applying a matching configuration:
# terraform import archestra_model_pool.balanced 11111111-1111-4111-8111-111111111111
