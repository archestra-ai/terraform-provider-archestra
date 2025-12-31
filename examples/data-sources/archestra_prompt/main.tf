data "archestra_prompt" "example" {
  name = "Coding Assistant Prompt"
}

output "prompt_content" {
  value = data.archestra_prompt.example.prompt
}
