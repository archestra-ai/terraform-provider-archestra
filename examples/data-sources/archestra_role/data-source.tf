# Look up an existing role by name
data "archestra_role" "developer" {
  name = "Developer"
}

# Reference the role's ID and permissions
output "developer_role_id" {
  value = data.archestra_role.developer.id
}

output "developer_permissions" {
  value = data.archestra_role.developer.permissions
}
