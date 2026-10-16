# Declaring a link deliberately exposes these resources to holders of its URL.
# The URL/token are sensitive and available only on creation. Import adopts
# metadata without recovering secrets. Input changes revoke/replace the link.
resource "archestra_skill" "shared" {
  name            = "shared-example"
  description     = "An explicitly shared skill"
  content         = "Example skill instructions."
  files           = {}
  labels          = []
  environment_ids = []
}
resource "archestra_marketplace_share_link" "example" {
  name           = "Explicitly shared skill"
  skill_ids      = [archestra_skill.shared.id]
  plugin_ids     = []
  expires_at     = "2040-01-01T00:00:00Z"
  rotation_nonce = "1"
  # Plugin links also require plugin_platform and expires_at; plugin_ids
  # accepts manual or repository package UUIDs. Expired links remain in state.
}
