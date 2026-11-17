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

  # "use" deploys every kind of object. "read" plus deploy-* actions limits a
  # grant to those kinds (requires Archestra 1.5.0-rc.37 or later).
  permissions = [{
    subject_type = "role"
    subject_id   = "member"
    actions      = ["read", "deploy-agent", "deploy-mcp-gateway"]
  }]
}
