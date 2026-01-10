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

data "archestra_user" "example" {
  id = "8ChlX1zlOGPlGvsAI6GE3AA1dr5dvd1d"
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
