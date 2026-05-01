resource "archestra_mcp_registry_catalog_item" "filesystem" {
  name        = "filesystem"
  description = "Filesystem MCP server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "filesystem" {
  name       = "filesystem"
  catalog_id = archestra_mcp_registry_catalog_item.filesystem.id
}

# One-line tool-id lookup via the install's `tool_id_by_name` map —
# no `data "archestra_mcp_server_tool"` plumbing required.
locals {
  file_write_tool_id = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__write_file"]
}

resource "archestra_tool_invocation_policy" "block_system_paths" {
  tool_id = local.file_write_tool_id
  conditions = [
    { key = "path", operator = "contains", value = "/etc/" },
  ]
  action = "block_always"
  reason = "Block writes to system configuration directories"
}

# Multi-condition policy — ALL conditions must match for `action` to fire.
resource "archestra_tool_invocation_policy" "block_dotfiles_in_home" {
  tool_id = local.file_write_tool_id
  conditions = [
    { key = "path", operator = "startsWith", value = "/home/" },
    { key = "path", operator = "contains", value = "/." },
  ]
  action = "block_always"
  reason = "Block writes that target hidden files under /home"
}
