# Wire a parent agent to delegate work to specialist sub-agents.
# Replaces the agent's full delegation set on every apply — drift in
# either direction (target added/removed out-of-band) surfaces as a
# normal plan diff.

resource "archestra_agent" "support_triage" {
  name        = "support-triage"
  description = "First-line support agent that routes by category."
}

resource "archestra_agent" "refunds" {
  name        = "refunds-specialist"
  description = "Handles refund requests."
}

resource "archestra_agent" "billing" {
  name        = "billing-specialist"
  description = "Handles billing questions."
}

resource "archestra_agent_delegation" "triage" {
  agent_id = archestra_agent.support_triage.id
  target_agent_ids = [
    archestra_agent.refunds.id,
    archestra_agent.billing.id,
  ]
}
