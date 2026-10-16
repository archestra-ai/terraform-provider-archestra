resource "archestra_team" "engineering" {
  name        = "Engineering"
  description = "Engineering team"
}

# Grant this team access to an object using archestra_resource_permission_policy.
# Team membership is managed through members and external group mappings.
resource "archestra_team" "support" {
  name        = "Support"
  description = "Support team"
}
