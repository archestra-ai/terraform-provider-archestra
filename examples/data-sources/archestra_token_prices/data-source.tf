# Fetch all configured token prices
data "archestra_token_prices" "all" {}

# Output the list of token prices
output "all_token_prices" {
  value = data.archestra_token_prices.all.token_prices
}

# Example: Find a specific model's pricing
output "gpt4o_pricing" {
  value = [
    for tp in data.archestra_token_prices.all.token_prices : tp
    if tp.model == "gpt-4o"
  ]
}
