variable "slack_bot_token" {
  type      = string
  sensitive = true
}

variable "slack_app_level_token" {
  type      = string
  sensitive = true
}

resource "archestra_chatops_slack" "this" {
  connection_mode = "socket"
  app_id          = "A0123456789"
  bot_token       = var.slack_bot_token
  app_level_token = var.slack_app_level_token
}
