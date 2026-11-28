# Map the built-in tools by name in one listing, instead of one archestra_tool
# lookup per tool.
data "archestra_tools" "builtin" {
  name_prefix = "archestra__"
}

# The tools of an installed MCP server come from that installation.
data "archestra_mcp_server" "kubernetes" {
  name = "kubernetes"
}

resource "archestra_agent" "operator" {
  name          = "Cluster operator"
  system_prompt = "Answer questions about the cluster."

  tools = concat(
    [for name in ["archestra__whoami", "archestra__todo_write"] : {
      tool_id                    = data.archestra_tools.builtin.tool_id_by_name[name]
      mcp_server_id              = null
      credential_resolution_mode = "static"
    }],
    [for name in ["kubernetes__kubectl_get", "kubernetes__kubectl_logs"] : {
      tool_id                    = data.archestra_mcp_server.kubernetes.tool_id_by_name[name]
      mcp_server_id              = data.archestra_mcp_server.kubernetes.id
      credential_resolution_mode = "static"
    }],
  )
}
