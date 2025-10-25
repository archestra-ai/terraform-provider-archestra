data "archestra_mcp_server_tool" "example" {
  mcp_server_id = "mcp-server-id-here"
  name          = "read_file"
}

output "tool_id" {
  value       = data.archestra_mcp_server_tool.example.id
  description = "The tool ID"
}

output "tool_description" {
  value = data.archestra_mcp_server_tool.example.description
}
