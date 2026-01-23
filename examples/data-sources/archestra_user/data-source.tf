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

# Create a user resource first
resource "archestra_user" "example" {
  name     = "Example User"
  email    = "example@test.com"
  password = "SecurePassword123!"
}

# Use the data source to read the created user
data "archestra_user" "example" {
  id = archestra_user.example.id
}

output "user_name" {
  value = data.archestra_user.example.name
}

output "user_email" {
  value = data.archestra_user.example.email
}

output "user_role" {
  value = data.archestra_user.example.role
}
