# Read an existing agent and its configuration as one object.
data "archestra_agent" "support" {
  name = "Support"
}

output "support_subagent_ids" {
  value = data.archestra_agent.support.subagent_ids
}

output "support_tools" {
  value = data.archestra_agent.support.tools
}

# For a stable identity, configure id instead of name.
# data "archestra_agent" "known" { id = "11111111-1111-4111-8111-111111111111" }
