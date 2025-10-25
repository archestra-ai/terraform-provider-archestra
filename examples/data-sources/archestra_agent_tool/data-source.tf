data "archestra_agent_tool" "example" {
  agent_id  = "agent-id-here"
  tool_name = "write_file"
}

output "agent_tool_id" {
  value       = data.archestra_agent_tool.example.id
  description = "Use this ID for creating policies"
}

output "tool_result_treatment" {
  value = data.archestra_agent_tool.example.tool_result_treatment
}
