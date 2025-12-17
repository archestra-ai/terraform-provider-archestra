resource "archestra_chat_llm_provider_api_key" "example" {
  name                    = "Production OpenAI Key"
  api_key                 = var.openai_api_key
  llm_provider            = "openai"
  is_organization_default = true
}
