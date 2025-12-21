data "archestra_prompt_versions" "example" {
  prompt_id = "prompt-123"
}

output "versions" {
  value = data.archestra_prompt_versions.example.versions
}
