variable "telegram_bot_token" {
  type      = string
  sensitive = true
}

resource "archestra_chatops_telegram" "this" {
  bot_token = var.telegram_bot_token
}
