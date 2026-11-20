variable "support_bot_token" {
  type      = string
  sensitive = true
}

variable "support_app_level_token" {
  type      = string
  sensitive = true
}

data "archestra_agent" "support" {
  name = "Support"
}

# A second Slack app whose bot always answers as the Support agent.
resource "archestra_chatops_slack_agent_bot" "support" {
  agent_id        = data.archestra_agent.support.id
  connection_mode = "socket"
  app_id          = "A0987654321"
  bot_token       = var.support_bot_token
  app_level_token = var.support_app_level_token
}
