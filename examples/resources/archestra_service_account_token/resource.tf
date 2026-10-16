resource "archestra_service_account_token" "example" {
  service_account_id = archestra_service_account.example.id
  name               = "Deployment credential"
  expires_in         = 86400
}
