# List every external IdP group currently mapped to a team. Useful for
# auditing IdP sync drift: compare the live list against your variable.
data "archestra_team_external_groups" "engineering" {
  team_id = archestra_team.engineering.id
}

output "engineering_external_group_count" {
  value = length(data.archestra_team_external_groups.engineering.groups)
}

# Surface group identifiers in a human-friendly map keyed by ID.
output "engineering_external_groups_by_id" {
  value = {
    for g in data.archestra_team_external_groups.engineering.groups :
    g.id => g.group_identifier
  }
}

# Diff against the desired set — drives an alert / failed plan if drift exists.
locals {
  desired_groups = toset(["sre-oncall", "platform-eng"])
  live_groups    = toset([for g in data.archestra_team_external_groups.engineering.groups : g.group_identifier])
}

output "external_group_drift" {
  description = "Groups present on the team but missing from the desired set."
  value       = setsubtract(local.live_groups, local.desired_groups)
}
