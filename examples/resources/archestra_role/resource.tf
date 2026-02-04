resource "archestra_role" "developer" {
  name        = "Developer"
  description = "Can manage agents and MCP servers"
  permissions = [
    "agents:read",
    "agents:write",
    "mcp_servers:read",
    "mcp_servers:write"
  ]
}

resource "archestra_role" "viewer" {
  name        = "Viewer"
  description = "Read-only access to agents and profiles"
  permissions = [
    "agents:read",
    "profiles:read"
  ]
}
