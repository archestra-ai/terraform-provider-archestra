# Map a team to an external HashiCorp Vault folder path. The team can
# only read secrets under that path. Enterprise Edition only.
#
# Each team can have at most one folder, so this resource is a
# per-team singleton; changing `team_id` forces replacement.

resource "archestra_team_vault_folder" "engineering" {
  team_id    = archestra_team.engineering.id
  vault_path = "secret/data/engineering"
}
