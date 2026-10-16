# One authoritative policy per internal agent. Native references use skill_id;
# external_mcp uses mcp_server_id + uri; plugin uses plugin_id + skill_path.
resource "archestra_agent_skill_policy" "restricted" {
  agent_id            = archestra_agent.worker.id
  mode                = "manual"
  allowed_references  = []
  excluded_references = []
}

# Import with the internal agent UUID. Destroy clears all references and resets
# mode to manual; it preserves the agent. Hidden/unavailable references must be
# resolved explicitly before this policy can be managed.
