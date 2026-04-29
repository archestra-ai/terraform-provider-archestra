# Single-tool default — matches the UI's per-row "Results are" dropdown.
# `mark_as_untrusted` is the wire name behind the UI's "Sensitive" label.
resource "archestra_trusted_data_policy_default" "echo_sensitive" {
  tool_ids = [archestra_mcp_server_installation.internal_test.tool_id_by_name["internal_test__echo"]]
  action   = "mark_as_untrusted"
}

# Bulk: sanitise every filesystem tool's result through the dual-LLM sanitiser.
resource "archestra_trusted_data_policy_default" "sanitize_filesystem" {
  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
  action   = "sanitize_with_dual_llm"
}
