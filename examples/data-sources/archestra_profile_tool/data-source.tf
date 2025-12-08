data "archestra_profile_tool" "example" {
  agent_id  = "agent-id-here"
  tool_name = "write_file"
}

output "profile_tool_id" {
  value       = data.archestra_profile_tool.example.id
  description = "Use this ID for creating policies"
}

output "tool_result_treatment" {
  value = data.archestra_profile_tool.example.tool_result_treatment
}
