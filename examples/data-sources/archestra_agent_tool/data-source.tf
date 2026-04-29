# Look up an agent's bound tool by name. Useful when the tool was assigned
# outside Terraform (UI, API) and you need its UUID to attach a policy.
data "archestra_agent_tool" "support_read_text_file" {
  agent_id  = archestra_agent.support.id
  tool_name = "read_text_file"
}

# Drive a tool-invocation policy off the lookup. `tool_id` on the policy
# resource is the bare UUID — not the agent-tool composite ID.
resource "archestra_tool_invocation_policy" "block_etc" {
  tool_id = data.archestra_agent_tool.support_read_text_file.tool_id
  conditions = [
    { key = "path", operator = "startsWith", value = "/etc/" },
  ]
  action = "block_always"
  reason = "Block reads from /etc/ regardless of trust level"
}

output "agent_tool_assignment_id" {
  description = "Composite agent-tool ID — useful in audit logs."
  value       = data.archestra_agent_tool.support_read_text_file.id
}
