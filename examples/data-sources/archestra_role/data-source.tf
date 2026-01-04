data "archestra_role" "admin" {
  name = "admin"
}

resource "archestra_user_role_assignment" "example" {
  user_id = "user-123"
  role_id = data.archestra_role.admin.id
}
