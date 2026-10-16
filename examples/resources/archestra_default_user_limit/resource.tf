resource "archestra_default_user_limit" "example" {
  environment_id   = archestra_environment.example.id
  limit_value      = 100
  cleanup_interval = "calendar_month"
}
