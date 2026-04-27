data "archestra_agent_tool" "file_write" {
  agent_id  = "00000000-0000-0000-0000-000000000000"
  tool_name = "write_file"
}

resource "archestra_tool_invocation_policy" "block_system_paths" {
  tool_id       = data.archestra_agent_tool.file_write.id
  argument_name = "path"
  operator      = "contains"
  value         = "/etc/"
  action        = "block_always"
  reason        = "Block writes to system configuration directories"
}
