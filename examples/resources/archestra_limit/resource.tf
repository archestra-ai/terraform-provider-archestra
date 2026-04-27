# Model-specific token cost limit (required for token_cost type)
resource "archestra_limit" "model_limit" {
  entity_id   = "your-organization-id"
  entity_type = "organization"
  limit_type  = "token_cost"
  limit_value = 500000
  model       = "gpt-4o"
}

# MCP server calls limit (requires mcp_server_name)
resource "archestra_limit" "mcp_limit" {
  entity_id       = archestra_team.engineering.id
  entity_type     = "team"
  limit_type      = "mcp_server_calls"
  limit_value     = 5000
  mcp_server_name = "github-mcp"
}

# Tool calls limit (requires both mcp_server_name and tool_name)
resource "archestra_limit" "tool_limit" {
  entity_id       = archestra_agent.assistant.id
  entity_type     = "agent"
  limit_type      = "tool_calls"
  limit_value     = 10000
  mcp_server_name = "github-mcp"
  tool_name       = "list_repos"
}
