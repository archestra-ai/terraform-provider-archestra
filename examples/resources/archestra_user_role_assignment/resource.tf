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
# CREATE: Assign a role to an existing user
# =============================================================================

# First, create a user
resource "archestra_user" "developer" {
  name     = "John Doe"
  email    = "john.doe@example.com"
  password = "SecurePassword123!"
}

# Then, create a role
resource "archestra_role" "temporary_role" {
  name = "temporary"

  permissions = {
    mcpServer = ["read"]
    team      = ["read"]
  }
}

# Assign the role to the user
resource "archestra_user_role_assignment" "developer_assignment" {
  user_id         = archestra_user.developer.id
  role_identifier = archestra_role.temporary_role.name
}

# =============================================================================
# READ: Reference assignment data using outputs
# =============================================================================
output "assignment_id" {
  description = "The ID of the role assignment"
  value       = archestra_user_role_assignment.developer_assignment.id
}

output "assigned_user_id" {
  description = "The user ID with the role assignment"
  value       = archestra_user_role_assignment.developer_assignment.user_id
}

output "assigned_role" {
  description = "The role assigned to the user"
  value       = archestra_user_role_assignment.developer_assignment.role_identifier
}

# =============================================================================
# UPDATE: Change the user's role
# =============================================================================
# To update the assigned role, modify role_identifier and run:
#   terraform plan   - to see what will change
#   terraform apply  - to apply the changes
#
# Note: Changing user_id will force a replacement of the resource

# =============================================================================
# IMPORT: Import an existing role assignment into Terraform state
# =============================================================================
# To import an existing role assignment by its ID:
#   terraform import archestra_user_role_assignment.developer_assignment <user-id>
