# Variables (declare in your variables.tf): organization_id (string).
# Externals (declare elsewhere): archestra_team.support, archestra_agent.support.

# Org-wide rule — anything under 500 tokens is cheap enough that gpt-4o-mini
# handles it. The rule fires when ALL `conditions` blocks match (logical AND
# across the array; logical OR within a single block's keys).
resource "archestra_optimization_rule" "short_prompts" {
  entity_type  = "organization"
  entity_id    = var.organization_id
  llm_provider = "openai"
  target_model = "gpt-4o-mini"
  enabled      = true

  conditions = [
    {
      max_length = 500
    }
  ]
}

# Tool-free traffic doesn't need Claude Sonnet — route to Haiku.
resource "archestra_optimization_rule" "no_tools_to_haiku" {
  entity_type  = "organization"
  entity_id    = var.organization_id
  llm_provider = "anthropic"
  target_model = "claude-haiku-4-5"
  enabled      = true

  conditions = [
    {
      has_tools = false
    }
  ]
}

# Team-scoped rule — only applies when the requesting agent belongs to the
# support team. Combines a length AND tool-presence check.
resource "archestra_optimization_rule" "support_short_no_tools" {
  entity_type  = "team"
  entity_id    = archestra_team.support.id
  llm_provider = "openai"
  target_model = "gpt-4o-mini"
  enabled      = true

  conditions = [
    {
      max_length = 1000
      has_tools  = false
    }
  ]
}

# Agent-scoped rule with multiple condition blocks (logical AND between them).
# Triggers when the prompt is short OR (in a separate condition entry) has no
# tools — both must hold for the rule to fire.
resource "archestra_optimization_rule" "support_agent_aggressive" {
  entity_type  = "agent"
  entity_id    = archestra_agent.support.id
  llm_provider = "openai"
  target_model = "gpt-4o-mini"
  enabled      = true

  conditions = [
    { max_length = 200 },
    { has_tools = false },
  ]
}

# Disabled rule kept around for audit — flip `enabled` back on without
# losing the configuration.
resource "archestra_optimization_rule" "snapshot_paused" {
  entity_type  = "organization"
  entity_id    = var.organization_id
  llm_provider = "openai"
  target_model = "gpt-4o-mini"
  enabled      = false

  conditions = [
    { max_length = 100 },
  ]
}
