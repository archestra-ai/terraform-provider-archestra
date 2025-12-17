# Look up a profile (agent) by name
data "archestra_profile" "default" {
  name = "Default Agent"
}

# Use the profile ID in other resources
output "profile_id" {
  value = data.archestra_profile.default.id
}

# Assign a tool to this profile
resource "archestra_profile_tool" "example" {
  profile_id = data.archestra_profile.default.id
  tool_id    = "tool-uuid-here" # Replace with actual tool ID

  use_dynamic_team_credential                = false
  allow_usage_when_untrusted_data_is_present = true
  tool_result_treatment                      = "trusted"
}
