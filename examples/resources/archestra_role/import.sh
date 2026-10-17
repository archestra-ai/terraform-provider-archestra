# Custom role IDs are base62 (not UUID). Find them in the platform UI
# at Settings -> Roles, or via `gh api .../roles`.
terraform import archestra_role.example abc123def456
