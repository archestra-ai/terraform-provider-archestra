# Read a team imported into the org outside Terraform — common when the team
# was created via the UI but you want to scope a TF-managed resource onto it.
data "archestra_team" "engineering" {
  id = var.engineering_team_id
}

# Use the team's id to scope an LLM provider key onto it.
resource "archestra_llm_provider_api_key" "engineering_anthropic" {
  name         = "Engineering Anthropic Key"
  api_key      = var.anthropic_key
  llm_provider = "anthropic"
  scope        = "team"
  team_id      = data.archestra_team.engineering.id
}

output "team_name" {
  value = data.archestra_team.engineering.name
}

# Members are surfaced as a list of { user_id, role } objects — useful for
# audit reports or for cross-checking that LDAP sync ran.
output "team_admins" {
  value = [for m in data.archestra_team.engineering.members : m.user_id if m.role == "admin"]
}
