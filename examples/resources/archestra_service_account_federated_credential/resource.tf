# Let a GitHub Actions workflow on `main` sign in as a service account with
# the job's own OIDC token, so the repository stores no Archestra API key.
#
# Every value matches exactly; there are no wildcards. To trust a second
# branch, workflow or environment, declare a second credential (a service
# account holds up to 20).
resource "archestra_service_account" "deployer" {
  name = "Deployment automation"
  role = "member"
}

resource "archestra_service_account_federated_credential" "example" {
  service_account_id = archestra_service_account.deployer.id
  name               = "acme/infra deploy workflow on main"
  issuer             = "https://token.actions.githubusercontent.com"
  # The job requests its token with this audience, usually the Archestra URL.
  audience = "https://archestra.example.com"
  subject  = "repo:acme/infra:ref:refs/heads/main"

  # Optional: other claims the token must carry. Pin the workflow file and
  # the repository's immutable ID, so a renamed or recreated repository of
  # the same name is not trusted.
  claim_conditions = {
    job_workflow_ref = "acme/infra/.github/workflows/deploy.yml@refs/heads/main"
    repository_id    = "123456789"
  }
}
