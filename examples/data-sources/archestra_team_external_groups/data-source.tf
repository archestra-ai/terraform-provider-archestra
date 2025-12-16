# List all external groups mapped to a team
data "archestra_team_external_groups" "example" {
  team_id = "team-id-here"
}

output "external_group_count" {
  value = length(data.archestra_team_external_groups.example.groups)
}

output "external_groups" {
  value = data.archestra_team_external_groups.example.groups
}
