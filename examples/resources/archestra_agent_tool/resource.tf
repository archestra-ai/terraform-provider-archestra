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

data "archestra_mcp_server_tool" "read_text_file" {
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
  name          = "${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"
}

resource "archestra_agent_tool" "read_text_file" {
  agent_id      = archestra_mcp_gateway.demo.id
  tool_id       = data.archestra_mcp_server_tool.read_text_file.id
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
}
