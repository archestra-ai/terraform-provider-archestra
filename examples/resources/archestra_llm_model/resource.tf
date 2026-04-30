# Override pricing for an existing model — useful when a discount agreement
# means the platform's auto-discovered prices undercount your real spend.
resource "archestra_llm_model" "gpt4o" {
  model_id = "gpt-4o"

  custom_price_per_million_input  = "2.50"
  custom_price_per_million_output = "10.00"
}

# Hide a model from the UI's model picker without deleting it (e.g. legacy
# models you no longer want users selecting). `ignored = true` is the wire
# equivalent of the UI's "ignore" toggle.
resource "archestra_llm_model" "deprecated_3_5" {
  model_id = "gpt-3.5-turbo"
  ignored  = true
}

# Override Claude with enterprise-contract pricing. The model's own
# `description` is read-only (carried from the provider catalog); only
# pricing, ignored, and modality overrides are settable on this resource.
resource "archestra_llm_model" "claude_sonnet" {
  model_id = "claude-sonnet-4-5"

  custom_price_per_million_input  = "2.40"
  custom_price_per_million_output = "12.00"
}

# Read-only outputs surfaced post-create — handy for debugging price drift.
output "gpt4o_effective_input_price" {
  value = archestra_llm_model.gpt4o.price_per_million_input
}

output "gpt4o_price_source" {
  description = "One of: custom, provider, default"
  value       = archestra_llm_model.gpt4o.price_source
}
