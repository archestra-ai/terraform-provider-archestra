resource "archestra_agent_hook" "example" {
  agent_id  = archestra_agent.example.id
  event     = "pre_tool_use"
  file_name = "validate.py"
  content   = "print(\"validate tool input\")"
}
