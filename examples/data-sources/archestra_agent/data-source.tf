# Lookup a known agent by ID. Works across all three agent variants
# (archestra_agent, archestra_llm_proxy, archestra_mcp_gateway) — check
# `agent_type` to discriminate.
data "archestra_agent" "support" {
  id = "11111111-1111-4111-a111-111111111111"
}

output "support_agent_name" {
  value = data.archestra_agent.support.name
}

output "support_team_count" {
  value = length(data.archestra_agent.support.team_ids)
}
