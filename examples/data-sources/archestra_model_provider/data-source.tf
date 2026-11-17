# A provider key created outside Terraform, found by its exact name.
data "archestra_model_provider" "by_name" {
  name = "Engineering Anthropic Key"
}

# The organization's primary key for one provider.
data "archestra_model_provider" "anthropic_primary" {
  llm_provider = "anthropic"
  is_primary   = true
}

# Use the key's ID where one is expected. The secret is never read.
resource "archestra_agent" "triage" {
  name           = "Issue triage"
  system_prompt  = "Label and route new issues."
  model_id       = data.archestra_model.claude_sonnet.id
  llm_api_key_id = data.archestra_model_provider.anthropic_primary.id
}

data "archestra_model" "claude_sonnet" {
  model_id     = "claude-sonnet-4-5"
  llm_provider = "anthropic"
}
