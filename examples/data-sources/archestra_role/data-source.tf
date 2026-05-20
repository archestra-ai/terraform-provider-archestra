# Look up a role by ID — accepts both base62 custom-role IDs and
# the literal name of a predefined role (`admin`, `member`, …).

data "archestra_role" "predefined_admin" {
  id = "admin"
}

output "admin_permissions" {
  value = data.archestra_role.predefined_admin.permission
}
