# All values seen for a specific label key — pair with the keys data
# source to validate that a policy `value` field references an
# already-used value.
data "archestra_agent_label_values" "team" {
  key = "team"
}

output "team_label_values" {
  value = data.archestra_agent_label_values.team.values
}

# Across-all-keys mode: omit `key` to get the union of every value used.
data "archestra_agent_label_values" "all" {}
