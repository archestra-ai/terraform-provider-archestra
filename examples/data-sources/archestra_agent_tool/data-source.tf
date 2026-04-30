# Look up an agent's bound tool by name. Useful when the tool was assigned
# outside Terraform (UI, API) and you need its UUID to attach a policy.
#
# The backend stores tool names in slugified `<prefix>__<raw>` form,
# where `<prefix>` is the catalog item's name for local installs and the
# install's name for remote installs. Easiest path: read the slugified
# name straight off the install (`archestra_mcp_server_installation.<n>.tools`
# or `tool_id_by_name`) so you don't have to know the prefix rule.
#
# `depends_on` is required when the agent-tool assignment is also managed
# in this module (e.g., via `archestra_agent_tool_batch.<n>`). Without
# it Terraform may run this data source before the assignment is
# created, surfacing a "Tool not found for agent" error.
data "archestra_agent_tool" "support_read_text_file" {
  agent_id  = archestra_agent.support.id
  tool_name = "${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"
  # depends_on = [archestra_agent_tool_batch.support_filesystem]
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
