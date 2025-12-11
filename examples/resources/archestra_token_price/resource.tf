# Manage token pricing for GPT-4o model
resource "archestra_token_price" "gpt4o" {
  model                    = "gpt-4o"
  price_per_million_input  = "2.50"
  price_per_million_output = "10.00"
}

# Manage token pricing for Claude 3 Opus
resource "archestra_token_price" "claude_opus" {
  model                    = "claude-3-opus-20240229"
  price_per_million_input  = "15.00"
  price_per_million_output = "75.00"
}

# Manage token pricing for a cheaper model
resource "archestra_token_price" "gpt4o_mini" {
  model                    = "gpt-4o-mini"
  price_per_million_input  = "0.15"
  price_per_million_output = "0.60"
}
