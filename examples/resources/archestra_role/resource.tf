# Custom RBAC role (Enterprise Edition only). Assign it to users
# through the platform UI or via your IdP's group mapping.
#
# Allowed actions per resource: admin, cancel, create, delete, enable,
# query, read, team-admin, update. Predefined roles (admin, member,
# owner, etc.) are managed by the backend and cannot be modified
# through this resource.
#
# Backend limitation: `description` cannot be cleared once set. The
# wire schema is non-nullable, and an empty string is silently dropped
# server-side. To remove a description, delete and recreate the role.
# Removing the attribute from HCL leaves the existing backend value
# untouched (merge-patch omits null transitions for this field).

resource "archestra_role" "support_reader" {
  name        = "Support Reader"
  description = "Read-only access to support agents and their MCP gateways."

  permission = {
    agent                 = ["read"]
    mcpServerInstallation = ["read"]
    mcpGateway            = ["read"]
  }
}

resource "archestra_role" "team_lead" {
  name        = "Team Lead"
  description = "Manage team membership and the team's agents."

  permission = {
    agent  = ["read", "create", "update", "delete"]
    team   = ["read", "team-admin"]
    apiKey = ["read", "create"]
  }
}
