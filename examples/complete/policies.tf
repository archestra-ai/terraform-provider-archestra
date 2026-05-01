# --- Default policies (bulk per-tool baseline) -----------------------------

resource "archestra_tool_invocation_policy_default" "filesystem_blocked" {
  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
  action   = "block_always"
}

resource "archestra_trusted_data_policy_default" "filesystem_sanitize" {
  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
  action   = "sanitize_with_dual_llm"
}

# LLM-driven policy generator over every memory tool. The backend writes
# default invocation + trusted-data policies and returns a per-tool reasoning
# string; the resource captures the analysis in state and never re-runs the
# LLM (changing tool_ids forces replacement).
resource "archestra_tool_policy_auto_config" "memory" {
  tool_ids = toset([for t in archestra_mcp_server_installation.memory.tools : t.id])
}

# --- Conditional invocation policies (layered on top of default) -----------

resource "archestra_tool_invocation_policy" "block_etc_writes" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__write_file"]
  conditions = [
    { key = "path", operator = "contains", value = "/etc/" },
  ]
  action = "block_always"
  reason = "Block writes to system configuration directories"
}

# Multi-block condition — separate condition entries (AND-across-blocks).
resource "archestra_tool_invocation_policy" "block_under_var_long_paths" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__write_file"]
  conditions = [
    { key = "path", operator = "startsWith", value = "/var/" },
    { key = "path", operator = "endsWith", value = ".log" },
  ]
  action = "block_always"
  reason = "Block .log writes anywhere under /var/ — multi-block AND condition"
}

# Args-based key — `key = "args.<param>"` targets a specific tool argument.
resource "archestra_tool_invocation_policy" "block_huge_head_reads" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
  conditions = [
    { key = "args.head", operator = "regex", value = "^[0-9]{4,}$" },
  ]
  action = "block_always"
  reason = "Block reads requesting 1000+ leading lines"
}

# Context-key policy — fires based on calling agent's trust context.
resource "archestra_tool_invocation_policy" "deny_writes_when_untrusted" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__write_file"]
  conditions = [
    { key = "context", operator = "equal", value = "untrusted" },
  ]
  action = "block_always"
  reason = "Hard deny for any write when the caller is in untrusted context"
}

resource "archestra_tool_invocation_policy" "filesystem_block_dotfiles" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__write_file"]
  conditions = [
    { key = "path", operator = "regex", value = "^/home/[^/]+/\\." },
  ]
  action = "block_always"
  reason = "Block writes to dotfiles under any user's home dir"
}

resource "archestra_tool_invocation_policy" "filesystem_block_root_paths" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__write_file"]
  conditions = [
    { key = "path", operator = "startsWith", value = "/" },
    { key = "path", operator = "notContains", value = "/tmp/" },
  ]
  action = "allow_when_context_is_untrusted"
  reason = "Allow non-/tmp absolute writes only when context is untrusted (audit trail)"
}

resource "archestra_tool_invocation_policy" "block_passwd" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
  conditions = [
    { key = "path", operator = "equal", value = "/etc/passwd" },
  ]
  action = "block_always"
  reason = "Specifically block /etc/passwd reads (operator: equal)"
}

resource "archestra_tool_invocation_policy" "block_secrets_keyword" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
  conditions = [
    { key = "path", operator = "contains", value = "secrets" },
  ]
  action = "block_always"
  reason = "Block reads of paths containing 'secrets' (operator: contains)"
}

resource "archestra_tool_invocation_policy" "block_pem_files" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
  conditions = [
    { key = "path", operator = "endsWith", value = ".pem" },
  ]
  action = "block_always"
  reason = "Block reads of .pem files (operator: endsWith)"
}

resource "archestra_tool_invocation_policy" "block_non_local_writes" {
  tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__write_file"]
  conditions = [
    { key = "path", operator = "notEqual", value = "/tmp/scratch.txt" },
  ]
  action = "allow_when_context_is_untrusted"
  reason = "Only the scratch file is allowed when context is untrusted (operator: notEqual)"
}

# --- Conditional trusted-data policies -------------------------------------

resource "archestra_trusted_data_policy" "trust_tmp_reads" {
  tool_id     = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
  description = "Treat reads from /tmp as trusted"
  conditions = [
    { key = "path", operator = "startsWith", value = "/tmp/" },
  ]
  action = "mark_as_trusted"
}

resource "archestra_trusted_data_policy" "filesystem_block_etc_reads" {
  tool_id     = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
  description = "Hard-block reads from /etc/ regardless of trust"
  conditions = [
    { key = "path", operator = "startsWith", value = "/etc/" },
  ]
  action = "block_always"
}

resource "archestra_trusted_data_policy" "filesystem_sanitize_var_log" {
  tool_id     = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
  description = "Sanitise /var/log/* through the dual-LLM filter"
  conditions = [
    { key = "path", operator = "startsWith", value = "/var/log/" },
  ]
  action = "sanitize_with_dual_llm"
}

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

resource "archestra_optimization_rule" "support_short_prompts" {
  entity_type  = "agent"
  entity_id    = archestra_agent.support.id
  llm_provider = "ollama"
  target_model = "llama3"
  enabled      = true

  conditions = [
    { max_length = 500 }
  ]
}

resource "archestra_optimization_rule" "support_batch_no_tools" {
  entity_type  = "agent"
  entity_id    = archestra_agent.support_batch.id
  llm_provider = "ollama"
  target_model = "llama3"
  enabled      = true

  conditions = [
    { has_tools = false }
  ]
}

resource "archestra_optimization_rule" "support_batch_short_no_tools" {
  entity_type  = "agent"
  entity_id    = archestra_agent.support_batch.id
  llm_provider = "ollama"
  target_model = "llama3"
  enabled      = true

  conditions = [
    { max_length = 1000, has_tools = false }
  ]
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

  depends_on = [archestra_agent_tool.read_text_file]
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
