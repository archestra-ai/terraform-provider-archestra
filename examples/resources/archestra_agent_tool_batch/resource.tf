# Assigns every tool from the filesystem MCP install onto the support agent
# in one round-trip. Equivalent to N `archestra_agent_tool` resources but
# uses the `bulk-assign` endpoint so plan/apply scales O(1) in N.
resource "archestra_agent_tool_batch" "support_agent_filesystem" {
  agent_id      = archestra_agent.support.id
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
  tool_ids      = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
  # Optional; defaults to "static". One of static | dynamic | enterprise_managed.
  credential_resolution_mode = "static"
}
