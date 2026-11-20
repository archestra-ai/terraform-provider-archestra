data "archestra_project" "onboarding" {
  name = "Onboarding"
}

output "onboarding_project_id" {
  value = data.archestra_project.onboarding.id
}
