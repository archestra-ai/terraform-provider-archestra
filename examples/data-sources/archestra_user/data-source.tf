terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
    }
  }
}

data "archestra_user" "example" {
  id = "user-id"
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

output "is_banned" {
  value = data.archestra_user.example.banned
}
