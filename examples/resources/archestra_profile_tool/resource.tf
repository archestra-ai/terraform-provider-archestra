# =============================================================================
# Example 1: Assign a built-in tool to a profile
# =============================================================================

# Create a Profile
resource "archestra_agent" "demo_agent" {
  name = "Demo Agent"
}

# Look up the built-in 'whoami' tool
data "archestra_profile_tool" "whoami" {
  tool_name  = "archestra__whoami"
  profile_id = archestra_agent.demo_agent.id
}

# Assign the Tool to the Profile
resource "archestra_profile_tool" "whoami" {
  profile_id = archestra_agent.demo_agent.id
  tool_id    = data.archestra_profile_tool.whoami.tool_id

  # Configuration Options
  tool_result_treatment                      = "trusted"
  allow_usage_when_untrusted_data_is_present = true

  # Dynamic team credentials can be toggled
  use_dynamic_team_credential = false

  # Optional: modify the tool response before it reaches the model
  response_modifier_template = "This is a modified response: {{.Result}}"
}

# =============================================================================
# Example 2: Assign an MCP Server tool with credentials configuration
# =============================================================================

# Create an MCP Server definition
resource "archestra_mcp_server" "filesystem" {
  name        = "filesystem-mcp-server"
  description = "MCP server for filesystem operations"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "./"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "filesystem" {
  name          = "my-filesystem-server"
  mcp_server_id = archestra_mcp_server.filesystem.id
}

# Look up a tool from the installed MCP server
data "archestra_mcp_server_tool" "read_text_file" {
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
  name          = "${archestra_mcp_server.filesystem.name}__read_text_file"
  depends_on    = [archestra_mcp_server_installation.filesystem]
}

# Assign the MCP tool to the profile with full configuration
resource "archestra_profile_tool" "read_text_file" {
  profile_id = archestra_agent.demo_agent.id
  tool_id    = data.archestra_mcp_server_tool.read_text_file.id

  # Specify which MCP server provides credentials
  credential_source_mcp_server_id = archestra_mcp_server_installation.filesystem.id

  # Specify which MCP server executes the tool
  execution_source_mcp_server_id = archestra_mcp_server_installation.filesystem.id

  # Use dynamic team credentials instead of user-specific credentials
  use_dynamic_team_credential = false

  # Allow tool usage even when untrusted data is present in the context
  allow_usage_when_untrusted_data_is_present = true

  # How to treat tool results: "trusted", "untrusted", or "sanitize_with_dual_llm"
  tool_result_treatment = "trusted"

  # Optional: Template to modify tool responses
  response_modifier_template = "File content: {{response}}"
}
