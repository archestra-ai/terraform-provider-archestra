# Grant a team access to an MCP server
resource "archestra_mcp_server_team_access" "example" {
  mcp_server_id = archestra_mcp_server.my_server.id
  team_id       = archestra_team.engineering.id
}
