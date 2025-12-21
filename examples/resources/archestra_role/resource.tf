resource "archestra_role" "developer" {
  name = "Developer"
  permissions = [
    "agents:read",
    "agents:create",
    "agents:update",
    "agents:delete",
  ]
}
