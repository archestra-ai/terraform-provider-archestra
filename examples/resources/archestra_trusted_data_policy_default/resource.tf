# Sanitise the result of every filesystem tool call through the dual-LLM
# sanitiser before flowing it back into the agent's context.
resource "archestra_trusted_data_policy_default" "sanitize_filesystem" {
  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
  action   = "sanitize_with_dual_llm"
}
