# An environment created outside this configuration, found by name.
data "archestra_environment" "production" {
  name = "Production"
}

output "production_environment_id" {
  value = data.archestra_environment.production.id
}
