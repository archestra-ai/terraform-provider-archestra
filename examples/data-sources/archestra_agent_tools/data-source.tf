# `depends_on` is required when assignments are also managed in this
# module (e.g., via `the agent tools attribute`). Without it
# Terraform may run this data source before the assignments exist and
# return an empty `tools` list.
data "archestra_agent_tools" "support_agent_tools" {
  agent_id = archestra_agent.support.id
  # depends_on = [archestra_agent.support]
}
