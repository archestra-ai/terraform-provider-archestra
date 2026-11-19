resource "archestra_service_account" "example" {
  name        = "Deployment automation"
  description = "Deploys agents from CI"
  role        = "member"
}
