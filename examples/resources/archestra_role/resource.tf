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
# CREATE: Define a new role with specific permissions
# =============================================================================
resource "archestra_role" "developer" {
  name = "test"

  permissions = {
    # Read-only access to MCP servers
    mcpServer = ["read"]

    # Read access to teams
    team = ["read"]
  }
}

# =============================================================================
# READ: Reference role data using outputs or data source
# =============================================================================
output "developer_role_id" {
  description = "The ID of the developer role"
  value       = archestra_role.developer.id
}

output "developer_permissions" {
  description = "Permissions assigned to developer role"
  value       = archestra_role.developer.permissions
}

output "is_predefined" {
  description = "Whether this is a predefined (built-in) role"
  value       = archestra_role.developer.predefined
}

# Use the data source to read an existing role
data "archestra_role" "existing" {
  id = archestra_role.developer.id
}

output "existing_role_id" {
  description = "The ID of the existing role"
  value       = data.archestra_role.existing.id
}

output "existing_permissions" {
  description = "Permissions assigned to existing role"
  value       = data.archestra_role.existing.permissions
}

# =============================================================================
# UPDATE: Modify the role by changing attributes
# =============================================================================
# To update a role, modify the resource definition and run:
#   terraform plan   - to see what will change
#   terraform apply  - to apply the changes

# resource "archestra_role" "developer" {
#   name = "test"

#   permissions = {
#     team = ["update"]
#   }
# }

# output "updatable_role_permissions" {
#   description = "Permissions assigned to updatable role"
#   value       = archestra_role.developer.permissions
# }

# =============================================================================
# IMPORT: Import an existing role into Terraform state
# =============================================================================
# To import an existing role by its ID:
#   terraform import archestra_role.developer <role-id>
