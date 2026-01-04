# Assign a role to a user
# Note: This resource requires backend API support for user role management
resource "archestra_user_role_assignment" "example" {
  user_id = "user-123"
  role_id = archestra_role.data_scientist.id
}

# Multiple role assignments
resource "archestra_user_role_assignment" "admin_assignment" {
  user_id = "user-456"
  role_id = "admin-role-id"
}
