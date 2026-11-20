# Let a GitHub Actions workflow on `main` sign in as an OAuth client with the
# job's own OIDC token, sent as the client assertion of a client credentials
# grant, so the repository stores no client secret.
#
# Every value matches exactly; there are no wildcards. To trust a second
# branch, workflow or environment, declare a second credential (a client
# holds up to 20). Only a client with grant_type "client_credentials" can
# have federated credentials.
resource "archestra_oauth_client" "deployer" {
  name       = "Deployment automation"
  scopes     = ["mcp"]
  grant_type = "client_credentials"
}

resource "archestra_oauth_client_federated_credential" "example" {
  oauth_client_id = archestra_oauth_client.deployer.id
  name            = "acme/infra deploy workflow on main"
  issuer          = "https://token.actions.githubusercontent.com"
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
