# First, register an MCP server in the private MCP registry
resource "archestra_mcp_registry_catalog_item" "filesystem" {
  name        = "filesystem-mcp-server"
  description = "MCP server for filesystem operations"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/home/user"]

    environment = {
      NODE_ENV = "production"
    }
  }
}

# Then, install the MCP server from the private registry
resource "archestra_mcp_server_installation" "example" {
  name          = "my-filesystem-server"
  mcp_server_id = archestra_mcp_server.filesystem.id
}
