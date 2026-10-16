resource "archestra_team" "engineering" {
  name = "engineering"
}

# The Vault folder must already exist. This maps the team to it without
# creating, reading, or changing Vault secrets.
resource "archestra_team_vault_folder" "engineering" {
  team_id    = archestra_team.engineering.id
  vault_path = "secret/data/teams/engineering"
}
