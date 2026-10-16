# --- Default policies (bulk per-tool baseline) -----------------------------




# --- Conditional invocation policies (layered on top of default) -----------











# --- Conditional trusted-data policies -------------------------------------




# --- Limits & optimisation rules -------------------------------------------

resource "archestra_limit" "engineering_filesystem_calls" {
  entity_id       = archestra_team.engineering.id
  entity_type     = "team"
  limit_type      = "mcp_server_calls"
  limit_value     = 5000
  mcp_server_name = archestra_mcp_server_installation.filesystem.name
}

resource "archestra_limit" "engineering_token_cost" {
  entity_id   = archestra_team.engineering.id
  entity_type = "team"
  limit_type  = "token_cost"
  limit_value = 250000
  model       = ["llama3"]
}

resource "archestra_limit" "support_read_text_calls" {
  entity_id       = archestra_agent.support.id
  entity_type     = "agent"
  limit_type      = "tool_calls"
  limit_value     = 100
  mcp_server_name = archestra_mcp_server_installation.filesystem.name
  tool_name       = "read_text_file"
}




# --- Data sources & outputs ------------------------------------------------

data "archestra_team" "engineering_lookup" {
  id = archestra_team.engineering.id
}

data "archestra_team_external_groups" "engineering_groups" {
  team_id = archestra_team.engineering.id
}

data "archestra_mcp_server_tool" "filesystem_write" {
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
  name          = "filesystem__write_file"
}

data "archestra_tool" "filesystem_write_global" {
  name       = "filesystem__write_file"
  depends_on = [archestra_mcp_server_installation.filesystem]
}

data "archestra_agent_tools" "support_lookup" {
  agent_id = archestra_agent.support.id
}

data "archestra_agent_tool" "support_read_text_file" {
  agent_id  = archestra_agent.support.id
  tool_name = "${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"

  depends_on = [archestra_agent.support]
}

data "archestra_mcp_tool_calls" "support_recent" {
  agent_id   = archestra_agent.support.id
  start_date = timeadd(timestamp(), "-24h")
  search     = "filesystem"
}

output "engineering_team_id" {
  value = archestra_team.engineering.id
}

output "engineering_team_name_via_data_source" {
  value = data.archestra_team.engineering_lookup.name
}

output "engineering_external_group_count" {
  value = length(data.archestra_team_external_groups.engineering_groups.groups)
}

output "filesystem_write_tool_id" {
  value = data.archestra_mcp_server_tool.filesystem_write.id
}

output "filesystem_write_tool_id_via_global_data_source" {
  value = data.archestra_tool.filesystem_write_global.id
}

output "support_agent_tool_count" {
  value = length(data.archestra_agent_tools.support_lookup.tools)
}

output "support_read_text_assignment_id" {
  value = data.archestra_agent_tool.support_read_text_file.id
}

output "support_tool_call_count_24h" {
  value = data.archestra_mcp_tool_calls.support_recent.total
}
