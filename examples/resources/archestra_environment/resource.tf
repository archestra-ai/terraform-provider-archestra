resource "archestra_environment" "staging" {
  name        = "Staging"
  description = "Staging deployments"

  trusted_image_registries = ["ghcr.io/example"]
  labels                   = [{ key = "stage", value = "staging" }]
  network_policy = {
    egress_mode     = "restricted"
    domain_preset   = "common_dependencies"
    allowed_domains = ["api.example.com"]
    allowed_cidrs   = []
  }
}
