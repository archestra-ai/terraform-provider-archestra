# Externals (declare elsewhere): archestra_mcp_server_installation.internal_test, archestra_mcp_server_installation.filesystem.

# Single-tool default — matches the UI's per-row dropdown.
resource "archestra_tool_invocation_policy_default" "echo_require_approval" {
  tool_ids = [archestra_mcp_server_installation.internal_test.tool_id_by_name["internal_test__echo"]]
  action   = "require_approval"
}

# Bulk default across every tool from one install.
resource "archestra_tool_invocation_policy_default" "filesystem_blocked" {
  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
  action   = "block_always"
}
