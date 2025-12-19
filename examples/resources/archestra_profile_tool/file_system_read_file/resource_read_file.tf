terraform {
  required_providers {
    archestra = {
      source = "registry.terraform.io/archestra-ai/archestra"
    }
  }
}

# 1. Create a Profile
resource "archestra_agent" "demo_agent" {
  name = "Demo Agent Filesystem"
}


resource "archestra_mcp_server" "filesystem" {
  name        = "filesystem-mcp-server"
  description = "MCP server for filesystem operations"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "./"]

  }
}

# Then, install the MCP server from the private registry
resource "archestra_mcp_server_installation" "example" {
  name          = "my-filesystem-server"
  mcp_server_id = archestra_mcp_server.filesystem.id
}

data "archestra_mcp_server_tool" "read_text_file" {
  mcp_server_id = archestra_mcp_server_installation.example.id
  name          = "${archestra_mcp_server.filesystem.name}__read_text_file"
  depends_on    = [archestra_mcp_server_installation.example]
}

# Assign the tool to the profile with full configuration
resource "archestra_profile_tool" "read_text_file" {
  profile_id = archestra_agent.demo_agent.id
  tool_id    = data.archestra_mcp_server_tool.read_text_file.id

  # Specify which MCP server provides credentials (defaults to the tool's MCP server)
  credential_source_mcp_server_id = archestra_mcp_server_installation.example.id

  # Specify which MCP server executes the tool (defaults to the tool's MCP server)
  execution_source_mcp_server_id = archestra_mcp_server_installation.example.id

  # Use dynamic team credentials instead of user-specific credentials
  use_dynamic_team_credential = false

  # Allow tool usage even when untrusted data is present in the context
  allow_usage_when_untrusted_data_is_present = true

  # How to treat tool results: "trusted", "untrusted", or "sanitize_with_dual_llm"
  tool_result_treatment = "trusted"

  # Optional: Template to modify tool responses
  response_modifier_template = "File content: {{response}}"
}
