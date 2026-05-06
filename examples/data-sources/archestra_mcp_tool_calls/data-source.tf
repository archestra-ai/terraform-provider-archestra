# All tool calls made by the support agent in the last 24 hours that
# mention "error" anywhere in server name, tool name, or arguments.
data "archestra_mcp_tool_calls" "support_failures_24h" {
  agent_id   = archestra_agent.support.id
  start_date = timeadd(timestamp(), "-24h")
  search     = "error"
}

output "support_failure_count_24h" {
  value = data.archestra_mcp_tool_calls.support_failures_24h.total
}

# Drive a downstream resource off the audit trail — e.g. raise an
# alert if any blocked-tool calls happened recently.
output "blocked_tool_calls" {
  value = [for c in data.archestra_mcp_tool_calls.support_failures_24h.calls : c if c.method == "tools/call"]
}
