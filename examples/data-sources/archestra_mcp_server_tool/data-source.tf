# Look up a single MCP server tool by name — the bare-UUID equivalent of the
# `archestra_mcp_server_installation.tool_id_by_name` map. Use when you need
# the data-source dependency edge instead of an attribute reference.
data "archestra_mcp_server_tool" "filesystem_read_text_file" {
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
  name          = "filesystem__read_text_file"
}


output "tool_id" {
  value       = data.archestra_mcp_server_tool.filesystem_read_text_file.id
  description = "Bare tool UUID — pass to policy resources' tool_id."
}

output "tool_description" {
  value = data.archestra_mcp_server_tool.filesystem_read_text_file.description
}
