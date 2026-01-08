# List all roles in the organization
data "archestra_roles" "all" {}

# Output all role names
output "all_role_names" {
  value = [for role in data.archestra_roles.all.roles : role.name]
}

# Filter to only custom roles
output "custom_roles" {
  value = [for role in data.archestra_roles.all.roles : role if !role.predefined]
}

# Filter to only predefined roles
output "predefined_roles" {
  value = [for role in data.archestra_roles.all.roles : role if role.predefined]
}
