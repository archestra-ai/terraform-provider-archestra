resource "archestra_profile_tool" "example" {
  profile_id = archestra_agent.example.id
  tool_id    = data.archestra_mcp_server_tool.example.id

  credential_source_mcp_server_id            = archestra_mcp_server.example.id
  execution_source_mcp_server_id             = archestra_mcp_server.example.id
  use_dynamic_team_credential                = false
  allow_usage_when_untrusted_data_is_present = false
  tool_result_treatment                      = "trusted"
}
