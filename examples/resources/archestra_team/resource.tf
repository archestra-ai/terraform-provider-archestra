resource "archestra_team" "engineering" {
  # Enterprise SSO settings belong to the team.
  external_group_ids = ["engineering"]

  name        = "Engineering"
  description = "Engineering team"
}

# Grant this team access through permissions on the target object.
# Team membership is managed through members and external group mappings.
resource "archestra_team" "support" {
  name        = "Support"
  description = "Support team"
}
