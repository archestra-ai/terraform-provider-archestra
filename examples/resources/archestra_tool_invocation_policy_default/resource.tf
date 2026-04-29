# Lock down every filesystem tool with a single resource. The bulk-default
# endpoint upserts the unconditional default policy for each tool_id; any
# conditional `archestra_tool_invocation_policy` resources targeting the
# same tools are evaluated first and fall through to this default.
resource "archestra_tool_invocation_policy_default" "filesystem_blocked" {
  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
  action   = "block_always"
}
