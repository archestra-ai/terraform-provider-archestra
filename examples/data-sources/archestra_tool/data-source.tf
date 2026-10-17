# Look up a tool by its exact name. Scope to an MCP server when available.
data "archestra_tool" "fs_read" {
  name          = "filesystem__read_text_file"
  mcp_server_id = archestra_mcp_server.filesystem.id
}

# Built-in tools can be found without a server selector.
data "archestra_tool" "builtin" {
  name = "archestra__whoami"
}

output "tool_id" { value = data.archestra_tool.fs_read.id }

# When managing the server, its tool_id_by_name map also exposes these UUIDs.
