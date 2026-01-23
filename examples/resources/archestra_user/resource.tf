terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
    }
  }
}

provider "archestra" {
  base_url = "http://localhost:8080"
}

# =============================================================================
# CREATE: Define a new user
# # =============================================================================
# resource "archestra_user" "developer" {
#   name     = "John Doe"
#   email    = "testing1@example.com"
#   password = "SecurePassword123!"
# }

# # User with optional profile image
# resource "archestra_user" "admin" {
#   name     = "Jane Admin"
#   email    = "testing2@example.com"
#   password = "AdminSecure456!"
#   image    = "https://example.com/profiles/jane.jpg"
# }

# =============================================================================
# READ: Reference user data using outputs or data source
# =============================================================================
# output "developer_user_id" {
#   description = "The ID of the developer user"
#   value       = archestra_user.developer.id
# }

# output "developer_email" {
#   description = "Email address of the developer user"
#   value       = archestra_user.developer.email
# }

# # Use the data source to read an existing user
# data "archestra_user" "existing" {
#   id = archestra_user.developer.id
# }

# output "existing_user_name" {
#   description = "The name of the existing user"
#   value       = data.archestra_user.existing.name
# }

# =============================================================================
# UPDATE: Modify the user by changing attributes
# =============================================================================
# To update a user, modify the resource definition and run:
#   terraform plan   - to see what will change
#   terraform apply  - to apply the changes

# Example: Update the user's name or email
resource "archestra_user" "developer" {
  name     = "John Doe"
  email    = "testing4@example.com"
  password = "SecurePassword123!"
}

# =============================================================================
# IMPORT: Import an existing user into Terraform state
# =============================================================================
# To import an existing user by its ID:
#   terraform import archestra_user.developer <user-id>
