# Look up a predefined role
data "archestra_role" "admin" {
  id = "admin"
}

# Look up a custom role by its ID
data "archestra_role" "custom" {
  id = "abc123xyz"
}

# Use the role's permissions
output "admin_permissions" {
  value = data.archestra_role.admin.permission
}
