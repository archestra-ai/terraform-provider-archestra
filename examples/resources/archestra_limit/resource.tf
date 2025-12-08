# Organization-wide token cost limit
resource "archestra_limit" "org_token_limit" {
  entity_id   = "your-organization-id"
  entity_type = "organization"
  limit_type  = "token_cost"
  limit_value = 1000000 # $1,000,000 limit
}

# Team-specific token cost limit
resource "archestra_limit" "team_token_limit" {
  entity_id   = archestra_team.engineering.id
  entity_type = "team"
  limit_type  = "token_cost"
  limit_value = 100000 # $100,000 limit
}

# Agent tool calls limit
resource "archestra_limit" "agent_tool_limit" {
  entity_id   = archestra_agent.assistant.id
  entity_type = "agent"
  limit_type  = "tool_calls"
  limit_value = 10000 # 10,000 tool calls
}

# MCP server calls limit for specific server
resource "archestra_limit" "mcp_limit" {
  entity_id       = archestra_team.engineering.id
  entity_type     = "team"
  limit_type      = "mcp_server_calls"
  limit_value     = 5000
  mcp_server_name = "github-mcp"
}

# Model-specific token cost limit
resource "archestra_limit" "model_limit" {
  entity_id   = "your-organization-id"
  entity_type = "organization"
  limit_type  = "token_cost"
  limit_value = 500000
  model       = "gpt-4o" # Only applies to GPT-4o usage
}
