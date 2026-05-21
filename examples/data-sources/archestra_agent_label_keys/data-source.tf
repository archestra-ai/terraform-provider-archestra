# Distinct label keys currently in use across agents. Use this to assert
# that a policy-condition key actually exists before referencing it —
# catches typos before they reach prod.
data "archestra_agent_label_keys" "all" {}

output "agent_label_keys" {
  value = data.archestra_agent_label_keys.all.keys
}

# Guard a tool-invocation policy on a label key the org actually uses.
locals {
  has_team_label = contains(data.archestra_agent_label_keys.all.keys, "team")
}
