# 1. Create an Agent (Profile)
resource "archestra_agent" "demo_agent" {
  name = "Demo Agent"
}

# 2. Look up the 'whoami' tool
data "archestra_agent_tool" "whoami" {
  tool_name = "archestra__whoami"
  agent_id  = archestra_agent.demo_agent.id
}

# 3. Assign the Tool to the Profile
resource "archestra_profile_tool" "demo_assignment" {
  profile_id = archestra_agent.demo_agent.id
  tool_id    = data.archestra_agent_tool.whoami.tool_id

  # Configuration Options
  tool_result_treatment                      = "trusted"
  allow_usage_when_untrusted_data_is_present = true

  # Note: dynamic team credentials can be toggled
  use_dynamic_team_credential = false

  response_modifier_template = "This is a modified response: {{.Result}}"
}
