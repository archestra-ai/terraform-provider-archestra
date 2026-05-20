# --- Custom RBAC roles (Enterprise Edition only) ---------------------------
#
# Predefined roles (`admin`, `member`, `owner`, etc.) are managed by the
# backend and not exposed here. This file declares two custom roles for
# the demo: a read-only support persona and a team lead with broader
# tool / api-key management. Assign them to users through the platform
# UI or via your IdP's group mapping.
#
# Allowed actions per resource: admin, cancel, create, delete, enable,
# query, read, team-admin, update. Resource keys are the backend's
# `resources` enum (agent, mcpGateway, mcpServerInstallation, team,
# apiKey, …); see platform/shared/permission.types.ts for the full list.

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
  description = "Manage agents and API keys within the team."

  # `team-admin` is intentionally omitted — only the platform owner
  # can grant it, and the demo admin account doesn't have it. Backend
  # returns 403 "You cannot grant permissions you don't have" if you
  # try to include actions outside the granting user's scope.
  permission = {
    agent  = ["read", "create", "update", "delete"]
    team   = ["read"]
    apiKey = ["read", "create"]
  }
}
