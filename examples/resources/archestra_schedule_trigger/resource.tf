# Run an agent on a cron schedule.
#
# `cron_expression` and `timezone` are validated by the backend; invalid
# values are rejected at apply time. Only internal agents
# (`agent_type = "agent"`) accept triggers — gateways and proxies
# return 400.

resource "archestra_schedule_trigger" "daily_summary" {
  name             = "Daily Engineering Summary"
  agent_id         = archestra_agent.support.id
  message_template = "Summarise the last 24 hours of GitHub issues into a Slack-ready digest."
  cron_expression  = "0 9 * * 1-5"
  timezone         = "America/New_York"
}

# Paused trigger — set `enabled = false` instead of deleting so you
# can resume by flipping it back to true without losing the config.
resource "archestra_schedule_trigger" "incident_drill" {
  name             = "Quarterly Incident Drill"
  agent_id         = archestra_agent.support.id
  message_template = "Simulate an SEV-1 customer outage; walk through the runbook end-to-end."
  cron_expression  = "0 14 1 */3 *"
  timezone         = "UTC"
  enabled          = false
}
