variable "github_app_private_key" {
  type      = string
  sensitive = true
}

resource "archestra_github_app_config" "example" {
  name            = "existing-source-app"
  app_id          = "12345"
  installation_id = "67890"
  private_key     = var.github_app_private_key
}
