resource "archestra_resource_permission_policy" "staging" {
  resource_type = "environment"
  resource_id   = archestra_environment.staging.id
  grants = [{
    subject_type = "organization"
    subject_id   = "*"
    actions      = ["read", "use"]
  }]
}
