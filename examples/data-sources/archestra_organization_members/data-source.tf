# List members of the organization. Useful for cross-referencing
# user IDs against IdP-synced groups or for auditing role
# assignments.

data "archestra_organization_members" "admins" {
  role = "admin"
}

output "admin_emails" {
  value = [for m in data.archestra_organization_members.admins.members : m.email]
}
