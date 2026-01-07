# Look up an existing role by name
data "archestra_role" "admin" {
  name = "admin"
}

# Look up a custom role by ID
data "archestra_role" "by_id" {
  id = "role-uuid-here"
}

# Output role permissions
output "admin_permissions" {
  value = data.archestra_role.admin.permissions
}
