resource "archestra_mcp_gateway" "demo" {
  name = "demo-mcp"
}

resource "archestra_mcp_registry_catalog_item" "filesystem" {
  name        = "filesystem"
  description = "Read-only filesystem MCP server"
  docs_url    = "https://github.com/modelcontextprotocol/servers"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "filesystem" {
  name       = "filesystem"
  catalog_id = archestra_mcp_registry_catalog_item.filesystem.id
}

# Look up the tool's UUID by name in one line.
# `tool_id_by_name` is keyed by the wire name `<server>__<short>`,
# so the lookup composes cleanly from the catalog item's name.
resource "archestra_agent_tool" "read_text_file" {
  agent_id      = archestra_mcp_gateway.demo.id
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
  tool_id       = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
}
