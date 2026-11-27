# ALPHA.
# Clients request the router's name as the model ("auto" or "auto:<name>").
variable "openai_provider_key_id" {
  type = string
}

# Tier models are archestra_model rows (database UUIDs, not provider model
# names). Each must be a tool-capable chat model on its tier's provider key.
resource "archestra_model" "fast" {
  model_id = "gpt-4.1-mini"
}
resource "archestra_model" "normal" {
  model_id = "gpt-4.1"
}
resource "archestra_model" "smart" {
  model_id = "o3"
}

resource "archestra_auto_router" "engineering" {
  name          = "auto:engineering"
  budget_preset = "balanced"

  tiers = {
    fast   = { model_id = archestra_model.fast.id, provider_api_key_id = var.openai_provider_key_id }
    normal = { model_id = archestra_model.normal.id, provider_api_key_id = var.openai_provider_key_id }
    smart  = { model_id = archestra_model.smart.id, provider_api_key_id = var.openai_provider_key_id }
  }

  # Optional; omit it for the platform's default decision model. The model
  # must be one the decisions provider (jev) offers; a null api_key_id uses
  # the organization's primary jev key.
  decision_model = {
    model = "jev-1.13.0"
  }
}

# A router on the default decision model can warm start from the
# organization's scored past sessions (learn_from_past_sessions, default
# true). Until its own traffic spans a week, its plans are sized at
# expected_traffic_factor times the organization's past daily traffic.
resource "archestra_auto_router" "support" {
  name                     = "auto:support"
  budget_preset            = "cost_efficient"
  learn_from_past_sessions = true
  expected_traffic_factor  = 1.5

  tiers = {
    fast   = { model_id = archestra_model.fast.id, provider_api_key_id = var.openai_provider_key_id }
    normal = { model_id = archestra_model.normal.id, provider_api_key_id = var.openai_provider_key_id }
    smart  = { model_id = archestra_model.smart.id, provider_api_key_id = var.openai_provider_key_id }
  }
}

# Run an agent on the router instead of a fixed model, key or pool.
resource "archestra_agent" "support" {
  name           = "Support"
  auto_router_id = archestra_auto_router.engineering.id
}

# Let a virtual key call the router by name.
resource "archestra_virtual_key" "ci" {
  name            = "CI"
  auto_router_ids = [archestra_auto_router.engineering.id]
}
