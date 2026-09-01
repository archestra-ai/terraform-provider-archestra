# Per-user credential — each person deposits their own value in the UI before
# they can start a run that requires it. Ideal for credentials that carry an
# individual's identity upstream (a personal GitHub PAT, a Claude Code
# subscription token).
resource "archestra_execution_credential" "github" {
  key         = "github"
  name        = "GitHub token"
  description = "A personal access token that can clone the repository and open pull requests."
}

# Organization-wide credential — one shared value serves every user. Deposit
# it directly from Terraform via `organization_value` (write-only: the backend
# never echoes it back, so rotations happen by changing this attribute).
resource "archestra_execution_credential" "openrouter" {
  key                = "openrouter"
  name               = "OpenRouter API key"
  description        = "Provider key for the multi-model PR self-review. Optional; reviews are skipped without it."
  allow_personal     = false
  allow_organization = true
  organization_value = var.openrouter_api_key
}

# Reference the definition from an agent's background execution by key:
#
#   background_execution = {
#     # ...
#     credentials = [
#       {
#         key           = "OPENROUTER_API_KEY"           # env var inside the run
#         scope         = "shared"                       # matches allow_organization
#         credential_id = archestra_execution_credential.openrouter.key
#         label         = "OpenRouter API key"
#         required      = false
#       },
#     ]
#   }
