resource "archestra_team" "example" {
  name            = "Engineering Team"
  description     = "Engineering team for production systems"
  organization_id = "org-123"
  created_by      = "user-456"

  members = [
    {
      user_id = "user-789"
      role    = "admin"
    },
    {
      user_id = "user-101"
      role    = "member"
    }
  ]
}
