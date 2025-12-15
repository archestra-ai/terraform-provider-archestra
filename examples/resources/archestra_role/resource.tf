resource "archestra_role" "example" {
  name        = "example-role"
  description = "A role for testing purposes"
  permissions = ["read:users", "write:users"]
}
