# Create a custom role with specific permissions
resource "archestra_role" "developer" {
  name = "Developer"

  permission = {
    # Full access to profiles
    profile = ["create", "read", "update", "delete"]

    # Read-only access to organization settings
    organization = ["read"]

    # Can manage their own interactions
    interaction = ["create", "read"]

    # Can use tools
    tool = ["read"]

    # Can manage MCP servers
    mcpServer = ["create", "read", "update", "delete"]
  }
}

# Create a read-only viewer role
resource "archestra_role" "viewer" {
  name = "Viewer"

  permission = {
    profile      = ["read"]
    organization = ["read"]
    interaction  = ["read"]
    team         = ["read"]
    prompt       = ["read"]
  }
}

# Create an admin-like role with most permissions
resource "archestra_role" "team_lead" {
  name = "Team Lead"

  permission = {
    profile      = ["create", "read", "update", "delete"]
    organization = ["read", "update"]
    team         = ["create", "read", "update", "delete"]
    member       = ["create", "read", "update", "delete"]
    invitation   = ["create", "read", "delete"]
    interaction  = ["create", "read", "update", "delete"]
    prompt       = ["create", "read", "update", "delete"]
    mcpServer    = ["create", "read", "update", "delete"]
    tool         = ["create", "read", "update", "delete"]
  }
}
