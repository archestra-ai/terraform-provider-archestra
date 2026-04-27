data "archestra_agent_tool" "example" {
  agent_id  = "00000000-0000-0000-0000-000000000000"
  tool_name = "read_text_file"
}

output "agent_tool_id" {
  description = "Use this in agent_tool_id on policy resources"
  value       = data.archestra_agent_tool.example.id
}
