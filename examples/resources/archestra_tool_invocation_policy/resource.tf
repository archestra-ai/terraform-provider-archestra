data "archestra_profile_tool" "file_write" {
  agent_id  = "agent-id-here"
  tool_name = "write_file"
}

resource "archestra_tool_invocation_policy" "block_system_paths" {
  profile_tool_id = data.archestra_profile_tool.file_write.id
  argument_name   = "path"
  operator        = "contains"
  value           = "/etc/"
  action          = "block_always"
  description     = "Block writes to system configuration directories"
}
