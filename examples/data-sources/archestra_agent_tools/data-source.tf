# `depends_on` is required when assignments are also managed in this
# module (e.g., via `archestra_agent_tool_batch.<n>`). Without it
# Terraform may run this data source before the assignments exist and
# return an empty `tools` list.
data "archestra_agent_tools" "support_agent_tools" {
  agent_id = archestra_agent.support.id
  # depends_on = [archestra_agent_tool_batch.support_filesystem]
}

# Fan out a per-tool invocation policy without listing tool names by hand.
resource "archestra_tool_invocation_policy" "block_unsafe" {
  for_each   = { for t in data.archestra_agent_tools.support_agent_tools.tools : t.tool_id => t }
  tool_id    = each.value.tool_id
  reason     = "Block ${each.value.name} when context is untrusted"
  conditions = [{ key = "context", operator = "equal", value = "untrusted" }]
  action     = "block_when_context_is_untrusted"
}
