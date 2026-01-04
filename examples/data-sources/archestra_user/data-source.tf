# Look up a user by ID
# Note: This data source requires backend API support for user queries
data "archestra_user" "example" {
  id = "user-123"
}

# Output user information
output "user_email" {
  value = data.archestra_user.example.email
}

output "user_role" {
  value = data.archestra_user.example.role
}
