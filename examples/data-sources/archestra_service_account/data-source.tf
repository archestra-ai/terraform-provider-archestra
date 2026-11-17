# A service account created outside this configuration, found by name.
data "archestra_service_account" "ci" {
  name = "ci-deploy"
}

# Let it run an agent, without pasting its ID.
resource "archestra_agent" "release_notes" {
  name          = "Release notes"
  system_prompt = "Write release notes from the merged pull requests."

  permissions = [{
    subject_type = "serviceAccount"
    subject_id   = data.archestra_service_account.ci.id
    actions      = ["read", "use"]
  }]
}
