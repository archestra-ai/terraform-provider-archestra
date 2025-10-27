terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
    }
  }
}

provider "archestra" {
  base_url = "http://localhost:9000"
  api_key  = var.archestra_api_key
}

variable "archestra_api_key" {
  description = "Archestra API key for authentication"
  type        = string
  sensitive   = true
}

# Create a team for organizing access control
resource "archestra_team" "platform_team" {
  name        = "Platform Team"
  description = "Team for managing platform infrastructure with Terraform"
}

# Create an agent for AI interactions
resource "archestra_agent" "terraform_agent" {
  name       = "terraform-managed-agent"
  is_demo    = false
  is_default = false
}

# Install an MCP server
resource "archestra_mcp_server_installation" "filesystem" {
  name = "filesystem-mcp"
}

# Look up a specific tool from the agent for creating policies
# Note: This requires the agent to have tools available. You may need to
# update the tool_name to match actual tools available on your agent.
data "archestra_agent_tool" "file_write" {
  agent_id  = archestra_agent.terraform_agent.id
  tool_name = "write_file"
}

# Create a tool invocation policy to block writes to system directories
resource "archestra_tool_invocation_policy" "block_system_paths" {
  agent_tool_id = data.archestra_agent_tool.file_write.id
  argument_name = "path"
  operator      = "contains"
  value         = "/etc/"
  action        = "block_always"
  reason        = "Block writes to system configuration directories"
}

# Create another policy to require approval for sensitive operations
resource "archestra_tool_invocation_policy" "require_approval_home" {
  agent_tool_id = data.archestra_agent_tool.file_write.id
  argument_name = "path"
  operator      = "contains"
  value         = "/home/"
  action        = "require_approval"
  reason        = "Writes to home directories require approval"
}

# Look up a tool that fetches external data for trusted data policies
# Note: Update tool_name if your agent uses a different tool
data "archestra_agent_tool" "fetch_url" {
  agent_id  = archestra_agent.terraform_agent.id
  tool_name = "fetch_url"
}

# Create a trusted data policy to mark certain domains as trusted
resource "archestra_trusted_data_policy" "trust_company_api" {
  agent_tool_id  = data.archestra_agent_tool.fetch_url.id
  description    = "Trust data from company API endpoints"
  attribute_path = "url"
  operator       = "contains"
  value          = "api.archestra.ai"
  action         = "mark_as_trusted"
}

# Outputs for easy reference
output "team_id" {
  value       = archestra_team.platform_team.id
  description = "The ID of the created team"
}

output "agent_id" {
  value       = archestra_agent.terraform_agent.id
  description = "The ID of the created agent"
}

output "mcp_server_id" {
  value       = archestra_mcp_server_installation.filesystem.id
  description = "The ID of the installed MCP server"
}
