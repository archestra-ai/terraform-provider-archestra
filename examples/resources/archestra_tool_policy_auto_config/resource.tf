# Run the platform's LLM-driven policy generator over every filesystem
# tool. The backend writes default invocation + trusted-data policies and
# returns a per-tool reasoning string; the resource captures the analysis
# in state and never re-runs the LLM (changing tool_ids forces replacement).
resource "archestra_tool_policy_auto_config" "filesystem" {
  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
}

output "filesystem_policy_reasoning" {
  description = "Why the LLM chose each policy — useful for audit / sign-off."
  value       = { for r in archestra_tool_policy_auto_config.filesystem.results : r.tool_id => r.reasoning }
}
