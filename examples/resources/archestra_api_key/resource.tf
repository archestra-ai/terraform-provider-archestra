# Platform `arch_...` API key used to authenticate to the Archestra
# API itself. The full token is returned only once at create time —
# Terraform stores it; the backend never echoes it again. Protect
# your state backend accordingly.
#
# Bootstrap pattern: use a root admin key (minted in the UI) to
# `terraform apply` the resources below, then switch downstream
# tooling to the issued per-application keys.

resource "archestra_api_key" "cicd" {
  name               = "ci-pipeline"
  expires_in_seconds = 60 * 60 * 24 * 365 # ~1 year (min: 86400 = 1 day)
}

# Non-expiring key (omit `expires_in_seconds`).
resource "archestra_api_key" "ops_admin" {
  name = "ops-admin"
}

# Pipe the issued token into your CI secrets store.
output "cicd_token" {
  value     = archestra_api_key.cicd.key
  sensitive = true
}
