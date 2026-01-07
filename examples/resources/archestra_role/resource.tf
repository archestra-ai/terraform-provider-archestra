# This example creates a custom RBAC role for developers
# with permissions to manage agents and read MCP servers

resource "archestra_role" "developer" {
  name = "Developer"
  permissions = {
    "agents"      = ["read", "create", "update"]
    "mcp_servers" = ["read"]
  }
}

# Example of a read-only viewer role
resource "archestra_role" "viewer" {
  name = "Viewer"
  permissions = {
    "agents"      = ["read"]
    "mcp_servers" = ["read"]
    "teams"       = ["read"]
  }
}
