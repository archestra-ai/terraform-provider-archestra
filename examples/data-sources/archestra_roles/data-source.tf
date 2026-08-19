# List roles in the organization, optionally filtered by a name
# substring.

data "archestra_roles" "all" {}

data "archestra_roles" "team_lead_variants" {
  name = "lead"
}

output "custom_role_ids" {
  # Predefined roles are excluded — they're managed by the backend
  # and you typically reference them by their literal name.
  value = [for r in data.archestra_roles.all.roles : r.id if !r.predefined]
}
