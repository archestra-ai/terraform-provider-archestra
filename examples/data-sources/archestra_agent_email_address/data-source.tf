# Look up the inbound email address provisioned for an agent. Useful for
# wiring downstream SMTP forwarders without copying the address by hand.
data "archestra_agent_email_address" "support" {
  agent_id = archestra_agent.support_bot.id
}

output "support_inbox" {
  value = data.archestra_agent_email_address.support.email_address
}

# Fail the plan loudly when incoming email is misconfigured at the org level.
# Surfaces a configuration-drift signal before the agent silently drops mail.
output "incoming_email_healthy" {
  value = (
    data.archestra_agent_email_address.support.provider_enabled
    && data.archestra_agent_email_address.support.agent_incoming_email_enabled
  )
}
