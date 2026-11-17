# Look up a model the platform discovered from a provider key, by the name the
# provider knows it by. `llm_provider` is only needed when several providers
# expose the same name.
data "archestra_model" "claude_sonnet" {
  model_id     = "claude-sonnet-4-5"
  llm_provider = "anthropic"
}

# Pin an agent to that model and to a key that exists already, without pasting
# either ID. `model_id` on the agent is the model's database UUID.
resource "archestra_agent" "release_notes" {
  name           = "Release notes"
  system_prompt  = "Write release notes from the merged pull requests."
  model_id       = data.archestra_model.claude_sonnet.id
  llm_api_key_id = data.archestra_model_provider.anthropic.id
}

data "archestra_model_provider" "anthropic" {
  llm_provider = "anthropic"
  is_primary   = true
}
