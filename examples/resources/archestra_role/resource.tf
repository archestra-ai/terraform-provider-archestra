# Create a custom RBAC role with specific permissions
# Note: This resource requires backend API support for role management
resource "archestra_role" "data_scientist" {
  name        = "Data Scientist"
  description = "Custom role for data science team members"
  permissions = [
    "data:read",
    "data:write",
    "model:train",
    "model:deploy",
    "experiment:create",
    "experiment:read"
  ]
}
