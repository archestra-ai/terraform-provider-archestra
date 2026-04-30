# Look up any tool by name — works for both built-in tools (the
# backend-provided `archestra__*` family) and MCP server tools.
#
# When to use which:
#   - `archestra_mcp_server_installation.<n>.tool_id_by_name["..."]`
#     — preferred for MCP tools when you already have the install
#     resource in scope. One line, no extra data source.
#   - `archestra_mcp_server_tool` — same shape as this, but scoped to
#     a specific install via `mcp_server_id`. Use when you want the
#     dependency edge to a single install.
#   - `archestra_tool` (this) — global lookup by name only. Reach for
#     it when the tool is a backend built-in (no install owns it) or
#     when you don't want to thread an install reference through the
#     module.
data "archestra_tool" "fs_read" {
  # MCP tool names are slugified `<catalog_item.name>__<short>`. For
  # built-ins, use the bare backend name (e.g., `archestra__whoami`).
  name       = "filesystem__read_text_file"
  depends_on = [archestra_mcp_server_installation.filesystem]
}

# Drive a tool-invocation policy off the lookup.
resource "archestra_tool_invocation_policy" "no_etc_reads" {
  tool_id = data.archestra_tool.fs_read.id
  conditions = [
    { key = "path", operator = "startsWith", value = "/etc/" },
  ]
  action = "block_always"
  reason = "Block reads under /etc/"
}

output "tool_id" {
  value       = data.archestra_tool.fs_read.id
  description = "Bare tool UUID — pass to policy resources' tool_id."
}

output "tool_description" {
  value = data.archestra_tool.fs_read.description
}
