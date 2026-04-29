# Token-cost limit at the org level — caps cumulative input+output token spend
# (in micro-USD, so 500_000 = $0.50). `model` is a list because a single limit
# can apply to several models simultaneously.
resource "archestra_limit" "org_token_cost" {
  entity_id   = var.organization_id
  entity_type = "organization"
  limit_type  = "token_cost"
  limit_value = 500000
  model       = ["gpt-4o", "gpt-4o-mini"]
}

# Per-team MCP-server-call ceiling — every team gets 5k GitHub MCP calls before
# requests start getting rate-limited. Requires `mcp_server_name`.
resource "archestra_limit" "engineering_github_calls" {
  entity_id       = archestra_team.engineering.id
  entity_type     = "team"
  limit_type      = "mcp_server_calls"
  limit_value     = 5000
  mcp_server_name = archestra_mcp_server_installation.github.name
}

# Per-agent tool-call quota — caps how many times the support agent can call
# `list_repos` on the github MCP. Requires both `mcp_server_name` and `tool_name`.
resource "archestra_limit" "support_list_repos" {
  entity_id       = archestra_agent.support.id
  entity_type     = "agent"
  limit_type      = "tool_calls"
  limit_value     = 200
  mcp_server_name = archestra_mcp_server_installation.github.name
  tool_name       = "list_repos"
}

# Multi-model token-cost cap — same dollar budget split across the Claude
# family. Useful when you want a single rolling spend window across models.
resource "archestra_limit" "claude_family_budget" {
  entity_id   = archestra_team.engineering.id
  entity_type = "team"
  limit_type  = "token_cost"
  limit_value = 250000
  model = [
    "claude-sonnet-4-5",
    "claude-opus-4-7",
    "claude-haiku-4-5",
  ]
}
