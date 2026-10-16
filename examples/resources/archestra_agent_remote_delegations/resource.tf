# Declare one authoritative external connection set per agent.
resource "archestra_agent_remote_delegations" "external" {
  agent_id       = archestra_agent.parent.id
  connection_ids = [archestra_remote_agent.external.connection_id]
}
