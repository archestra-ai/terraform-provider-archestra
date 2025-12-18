data "archestra_dual_llm_config" "example" {
  id = "config-id-here"
}

output "dual_llm_config_enabled" {
  value = data.archestra_dual_llm_config.example.enabled
}

output "main_agent_prompt" {
  value = data.archestra_dual_llm_config.example.main_agent_prompt
}

