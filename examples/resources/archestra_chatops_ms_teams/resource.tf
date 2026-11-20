variable "teams_app_secret" {
  type      = string
  sensitive = true
}

resource "archestra_chatops_ms_teams" "this" {
  app_id     = "00000000-0000-0000-0000-000000000000"
  tenant_id  = "11111111-1111-1111-1111-111111111111"
  app_secret = var.teams_app_secret
}
