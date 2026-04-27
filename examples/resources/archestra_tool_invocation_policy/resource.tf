data "archestra_agent_tool" "file_write" {
  agent_id  = "00000000-0000-0000-0000-000000000000"
  tool_name = "write_file"
}

resource "archestra_tool_invocation_policy" "block_system_paths" {
  tool_id = data.archestra_agent_tool.file_write.id
  conditions = [
    { key = "path", operator = "contains", value = "/etc/" },
  ]
  action = "block_always"
  reason = "Block writes to system configuration directories"
}

# Multi-condition policy — ALL conditions must match for `action` to fire.
resource "archestra_tool_invocation_policy" "block_dotfiles_in_home" {
  tool_id = data.archestra_agent_tool.file_write.id
  conditions = [
    { key = "path", operator = "startsWith", value = "/home/" },
    { key = "path", operator = "contains", value = "/." },
  ]
  action = "block_always"
  reason = "Block writes that target hidden files under /home"
}
