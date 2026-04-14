# Manage custom pricing for an LLM model
resource "archestra_llm_model" "gpt4o" {
  model_id = "gpt-4o"

  # Override provider pricing with custom rates
  custom_price_per_million_input  = "2.50"
  custom_price_per_million_output = "10.00"
}

# Ignore a model (hide from model selection)
resource "archestra_llm_model" "ignored" {
  model_id = "gpt-3.5-turbo"
  ignored  = true
}
