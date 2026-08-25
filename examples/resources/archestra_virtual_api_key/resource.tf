# A virtual API key issues a delegate token tied to a parent
# `archestra_llm_provider_api_key`. The full token is returned only once
# at create time — Terraform stores it in state; the backend never echoes
# it again. Treat `value` like an AWS access key.
#
# --- Variables this file references ---
# Add to your variables.tf:
#
#   variable "anthropic_api_key" { type = string, sensitive = true }
#
# Also assumed declared elsewhere: archestra_team.engineering,
# archestra_team.support.

# Parent provider key the virtual key delegates from.
resource "archestra_llm_provider_api_key" "shared" {
  name         = "Shared Anthropic Key"
  api_key      = var.anthropic_api_key
  llm_provider = "anthropic"
}

# Org-scoped virtual key — visible to every member.
resource "archestra_virtual_api_key" "org_default" {
  llm_provider_api_key_id = archestra_llm_provider_api_key.shared.id
  name                    = "Shared CI/CD Key"
  scope                   = "org"
}

# Team-scoped virtual key with an expiration. `expires_at` must be
# RFC 3339. Omit for a non-expiring key.
resource "archestra_virtual_api_key" "engineering_quarterly" {
  llm_provider_api_key_id = archestra_llm_provider_api_key.shared.id
  name                    = "Engineering Q1 2027 Key"
  scope                   = "team"
  teams                   = [archestra_team.engineering.id, archestra_team.support.id]
  expires_at              = "2027-04-01T00:00:00Z"
}

# Pipe the issued token into your CI secrets store or downstream tool.
# Terraform marks `value` Sensitive, so it won't print in plan/apply
# output but it WILL land in state — protect your state backend.
output "ci_virtual_key" {
  value     = archestra_virtual_api_key.org_default.value
  sensitive = true
}
