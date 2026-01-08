resource "archestra_role" "developer" {
  name = "Developer"
  permissions = {
    "agents"      = ["read", "update"]
    "mcp_servers" = ["read"]
    "teams"       = ["read"]
  }
}

# Output the created role ID
output "developer_role_id" {
  value = archestra_role.developer.id
}
