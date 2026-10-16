# One authoritative exclusion set per agent. Exclusions persist while Auto
# delegation is disabled; destroy clears them while preserving the agent.
resource "archestra_agent_subagent_exclusions" "worker" {
  agent_id              = archestra_agent.worker.id
  excluded_subagent_ids = [archestra_agent.restricted.id]
}
