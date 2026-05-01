# Engineering team — the simplest case: name + description.
resource "archestra_team" "engineering" {
  name        = "Engineering"
  description = "Engineering team for production systems"
}

# Support team. To enable per-team TOON tool-result compression, also set
# `archestra_organization_settings.compression_scope = "team"` and add a
# `depends_on` on this team so org_settings applies before the team in
# the same pass — backend silently ignores team-level writes otherwise.
#
# TODO(backend): drop the `depends_on` requirement once the platform's
# `CreateTeamBodySchema` accepts `convertToolResultsToToon` and the team
# endpoint honors the flag regardless of org scope.
resource "archestra_team" "support" {
  name        = "Support"
  description = "Customer support team"
  # convert_tool_results_to_toon = true
  # depends_on                   = [archestra_organization_settings.main]
}

# Once a team exists, downstream resources can scope to it:
#   - archestra_agent.scope = "team", teams = [archestra_team.engineering.id]
#   - archestra_llm_provider_api_key.scope = "team", team_id = ...
#   - archestra_team_external_group.team_id = archestra_team.engineering.id
output "engineering_team_id" {
  value = archestra_team.engineering.id
}
