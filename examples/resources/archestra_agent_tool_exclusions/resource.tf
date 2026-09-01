# Externals (declare elsewhere): archestra_mcp_server_installation.files.

# An Auto-tool-mode agent sees every tool it can reach; exclusions carve
# specific tools back out. This resource owns the agent's ENTIRE exclusion
# list — declare at most one per agent, and manage every exclusion here
# (out-of-band additions are reverted on the next apply).
resource "archestra_agent" "runner" {
  name             = "repo-runner"
  system_prompt    = "You run delegated repository work."
  access_all_tools = true
}

resource "archestra_agent_tool_exclusions" "runner" {
  agent_id = archestra_agent.runner.id

  # Bare tool UUIDs. `tool_id_by_name` on an installation is the ergonomic
  # lookup; an empty set is valid and clears all exclusions.
  excluded_tool_ids = [
    archestra_mcp_server_installation.files.tool_id_by_name["files__delete_file"],
  ]
}
