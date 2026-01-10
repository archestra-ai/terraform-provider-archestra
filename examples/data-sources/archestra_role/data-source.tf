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

data "archestra_role" "example" {
  id = "admin"
}

output "role_name" {
  value = data.archestra_role.example.name
}

output "role_description" {
  value = data.archestra_role.example.description
}

output "role_permissions" {
  value = data.archestra_role.example.permissions
}
