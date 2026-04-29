# Engineering team — the simplest case: name + description.
resource "archestra_team" "engineering" {
  name        = "Engineering"
  description = "Engineering team for production systems"
}

# Support team. The `convert_tool_results_to_toon` line is commented out
# because it requires a precondition the example can't see — see TODO below.
#
# TODO(backend): team-level `convert_tool_results_to_toon` is silently
# ignored by the backend unless `archestra_organization_settings.compression_scope = "team"`.
# When ignored, the backend echoes `false` and Terraform errors with
# "Provider produced inconsistent result after apply". The provider
# pre-flights GetOrganization on Create/Update and refuses with a clear
# precondition error, but a fresh paste (without org_settings declared yet)
# would still trip the pre-flight. Uncomment the line below after applying
# `archestra_organization_settings { compression_scope = "team" }`. Once
# the backend honors the team-level flag regardless of scope (or rejects
# team-level writes with a 4xx when scope != "team"), drop the pre-flight
# check in resource_team.go and uncomment by default.
resource "archestra_team" "support" {
  name        = "Support"
  description = "Customer support team"
  # convert_tool_results_to_toon = true
}

# Once a team exists, downstream resources can scope to it:
#   - archestra_agent.scope = "team", teams = [archestra_team.engineering.id]
#   - archestra_llm_provider_api_key.scope = "team", team_id = ...
#   - archestra_team_external_group.team_id = archestra_team.engineering.id
output "engineering_team_id" {
  value = archestra_team.engineering.id
}
