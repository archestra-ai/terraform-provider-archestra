data "archestra_team" "example" {
  id = "team-id-here"
}

output "team_name" {
  value = data.archestra_team.example.name
}

output "team_members" {
  value = data.archestra_team.example.members
}
