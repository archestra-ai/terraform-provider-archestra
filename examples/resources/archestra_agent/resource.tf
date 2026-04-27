resource "archestra_agent" "support" {
  name          = "customer-support"
  description   = "Tier-1 customer support agent"
  system_prompt = "You are a friendly customer-support agent."
  llm_model     = "gpt-4o"
  scope         = "org"

  suggested_prompts = [
    {
      summary_title = "Refund a charge"
      prompt        = "Help me refund a customer charge."
    }
  ]

  labels = [
    { key = "team", value = "support" }
  ]
}
