resource "archestra_agent_knowledge_source_exclusions" "example" {
  agent_id               = archestra_agent.example.id
  excluded_connector_ids = ["11111111-1111-4111-8111-111111111111"]
}
