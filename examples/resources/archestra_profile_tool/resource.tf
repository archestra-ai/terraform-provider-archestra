# Example 1: Complete profile tool assignment with all options

# Look up an existing profile
data "archestra_profile" "default" {
  name = "Default Agent"
}

# Create or reference an MCP server
resource "archestra_mcp_server" "github" {
  name      = "github-server"
  transport = "stdio"
  command   = "npx"
  args      = ["-y", "@modelcontextprotocol/server-github"]

  environment = {
    GITHUB_PERSONAL_ACCESS_TOKEN = var.github_token
  }
}

# Look up a tool from the MCP server
data "archestra_mcp_server_tool" "github_create_issue" {
  mcp_server_id = archestra_mcp_server.github.id
  name          = "create_issue"
}

# Assign the tool to the profile with full configuration
resource "archestra_profile_tool" "github_create_issue" {
  profile_id = data.archestra_profile.default.id
  tool_id    = data.archestra_mcp_server_tool.github_create_issue.id

  # Specify which MCP server provides credentials (defaults to the tool's MCP server)
  credential_source_mcp_server_id = archestra_mcp_server.github.id

  # Specify which MCP server executes the tool (defaults to the tool's MCP server)
  execution_source_mcp_server_id = archestra_mcp_server.github.id

  # Use dynamic team credentials instead of user-specific credentials
  use_dynamic_team_credential = false

  # Allow tool usage even when untrusted data is present in the context
  allow_usage_when_untrusted_data_is_present = true

  # How to treat tool results: "trusted", "untrusted", or "sanitize_with_dual_llm"
  tool_result_treatment = "trusted"

  # Optional: Template to modify tool responses
  response_modifier_template = "Issue created: {{response}}"
}

# Example 2: Minimal profile tool assignment
resource "archestra_profile_tool" "minimal" {
  profile_id = data.archestra_profile.default.id
  tool_id    = data.archestra_mcp_server_tool.github_create_issue.id

  # All other fields will use defaults:
  # - credential_source_mcp_server_id: tool's MCP server
  # - execution_source_mcp_server_id: tool's MCP server
  # - use_dynamic_team_credential: false
  # - allow_usage_when_untrusted_data_is_present: true
  # - tool_result_treatment: "trusted"
}
