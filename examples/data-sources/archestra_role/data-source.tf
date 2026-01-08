# Look up an existing/system role by name
data "archestra_role" "admin" {
  name = "admin"
}

output "admin_role_id" {
  value = data.archestra_role.admin.id
}

output "admin_role_permissions" {
  value = data.archestra_role.admin.permissions
}

output "admin_is_predefined" {
  value = data.archestra_role.admin.predefined
}
