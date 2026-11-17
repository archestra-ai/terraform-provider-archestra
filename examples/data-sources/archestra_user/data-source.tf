# A member of the organization, found by email address.
data "archestra_user" "release_manager" {
  email = "release-manager@example.com"
}

# Grant that person access to an agent without pasting their user ID.
resource "archestra_agent" "release_notes" {
  name          = "Release notes"
  system_prompt = "Write release notes from the merged pull requests."

  permissions = [{
    subject_type = "user"
    subject_id   = data.archestra_user.release_manager.id
    actions      = ["read", "use"]
  }]
}
