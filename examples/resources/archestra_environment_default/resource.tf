resource "archestra_environment" "apps" {
  name = "apps"
}

# Manage one default slot per resource type. Destroy restores the implicit
# Default environment for apps and preserves the other resource defaults.
resource "archestra_environment_default" "apps" {
  resource_type  = "app"
  environment_id = archestra_environment.apps.id
}
