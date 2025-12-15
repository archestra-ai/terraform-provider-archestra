resource "archestra_user_role_assignment" "example" {
  user_id = archestra_user.example.id
  role_id = archestra_role.example.id
}
