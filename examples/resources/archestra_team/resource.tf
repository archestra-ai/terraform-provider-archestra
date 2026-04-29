# Engineering team — the simplest case: name + description.
resource "archestra_team" "engineering" {
  name        = "Engineering"
  description = "Engineering team for production systems"
}

# Support team with TOON tool-result compression enabled. The team-level flag
# overrides the org default from `archestra_organization_settings`.
resource "archestra_team" "support" {
  name                         = "Support"
  description                  = "Customer support team"
  convert_tool_results_to_toon = true
}

# Once a team exists, downstream resources can scope to it:
#   - archestra_agent.scope = "team", teams = [archestra_team.engineering.id]
#   - archestra_llm_provider_api_key.scope = "team", team_id = ...
#   - archestra_team_external_group.team_id = archestra_team.engineering.id
output "engineering_team_id" {
  value = archestra_team.engineering.id
}
