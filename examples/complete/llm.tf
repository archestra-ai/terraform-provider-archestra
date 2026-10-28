resource "archestra_model_provider" "ollama" {
  name         = "Local Ollama"
  llm_provider = "ollama"
  api_key      = "unused"
}

resource "archestra_model" "llama3" {
  model_id                        = "llama3"
  custom_price_per_million_input  = "0.50"
  custom_price_per_million_output = "1.00"
  ignored                         = true # hide from UI picker; agents that reference llama3 by id still work
}
