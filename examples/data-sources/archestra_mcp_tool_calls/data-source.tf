# Tool calls made by the support agent since a fixed instant. Filter the
# `calls` list in Terraform; `returned_count` is the number of calls returned
# and `truncated` says whether the window held more. Use a fixed timestamp (a
# variable here): a window built from timestamp() defers the read to apply time
# and leaves every plan non-empty.
data "archestra_mcp_tool_calls" "support_failures" {
  agent_id   = archestra_agent.support.id
  start_date = var.audit_window_start
}

output "support_tool_call_count" {
  value = data.archestra_mcp_tool_calls.support_failures.returned_count
}

# Drive a downstream resource off the audit trail — e.g. raise an
# alert if any blocked-tool calls happened recently.
output "blocked_tool_calls" {
  value = [for c in data.archestra_mcp_tool_calls.support_failures.calls : c if c.method == "tools/call"]
}
