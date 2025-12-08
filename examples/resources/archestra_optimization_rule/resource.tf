# Switch to cheaper model for short prompts
resource "archestra_optimization_rule" "short_prompts" {
  entity_type  = "organization"
  entity_id    = "your-organization-id"
  llm_provider = "openai"
  target_model = "gpt-4o-mini"
  enabled      = true

  conditions = [
    {
      max_length = 500 # Use cheaper model for prompts under 500 tokens
    }
  ]
}

# Use cheaper model when no tools are needed
resource "archestra_optimization_rule" "no_tools" {
  entity_type  = "organization"
  entity_id    = "your-organization-id"
  llm_provider = "anthropic"
  target_model = "claude-3-haiku-20240307"
  enabled      = true

  conditions = [
    {
      has_tools = false # Use cheaper model when no tools are present
    }
  ]
}

# Team-specific optimization rule
resource "archestra_optimization_rule" "team_optimization" {
  entity_type  = "team"
  entity_id    = archestra_team.support.id
  llm_provider = "openai"
  target_model = "gpt-4o-mini"
  enabled      = true

  conditions = [
    {
      max_length = 1000
    }
  ]
}

# Agent-specific optimization with multiple conditions
resource "archestra_optimization_rule" "agent_optimization" {
  entity_type  = "agent"
  entity_id    = archestra_agent.chatbot.id
  llm_provider = "openai"
  target_model = "gpt-4o-mini"
  enabled      = true

  conditions = [
    {
      max_length = 200
    },
    {
      has_tools = false
    }
  ]
}
