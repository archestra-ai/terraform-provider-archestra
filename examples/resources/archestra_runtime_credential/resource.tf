# Per-user credential — each person deposits their own value in the UI before
# they can start a run that requires it. Ideal for credentials that carry an
# individual's identity upstream (a personal GitHub PAT, a Claude Code
# subscription token).
resource "archestra_runtime_credential" "github" {
  key         = "github"
  name        = "GitHub token"
  description = "A personal access token that can clone the repository and open pull requests."
}

# Organization-wide credential — one shared value serves every user. Deposit
# it directly from Terraform via `organization_value` (write-only: the backend
# never echoes it back, so rotations happen by changing this attribute).
resource "archestra_runtime_credential" "openrouter" {
  key                = "openrouter"
  name               = "OpenRouter API key"
  description        = "Provider key for the multi-model PR self-review. Optional; reviews are skipped without it."
  allow_personal     = false
  allow_organization = true
  organization_value = var.openrouter_api_key
}

# Reference the definition from an Agent's dedicated runtime by key:
#
#   runtime = {
#     # ...
#     credentials = [
#       {
#         key           = "OPENROUTER_API_KEY"           # env var inside the run
#         scope         = "shared"                       # matches allow_organization
#         credential_id = archestra_runtime_credential.openrouter.key
#         label         = "OpenRouter API key"
#         required      = false
#       },
#     ]
#   }

# One organization App provides repository access. Deposit its private key and
# OAuth client secret through Credentials so secret material stays out of state.
resource "archestra_runtime_credential" "github_app" {
  key                = "example-github-app"
  name               = "Example GitHub App"
  kind               = "github_app"
  allow_personal     = false
  allow_organization = true
  app_id             = "12345"
  installation_id    = "67890"
  github_url         = "https://api.github.com"
  github_client_id   = "example-client-id"
}

# Each person authorizes this connection once, then every agent can reuse it.
resource "archestra_runtime_credential" "github_user" {
  key                       = "example-github-user"
  name                      = "GitHub account"
  kind                      = "github_app_user"
  github_app_credential_key = archestra_runtime_credential.github_app.key
}
