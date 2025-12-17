package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccProfileToolResource(t *testing.T) {
	t.Skip("Skipping test - requires actual MCP server and tool setup in test environment")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccProfileToolResourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile_tool.test",
						tfjsonpath.New("profile_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile_tool.test",
						tfjsonpath.New("tool_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile_tool.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
				},
			},
			// ImportState testing - commented out for unit test
			// Import can be tested manually with:
			// terraform import archestra_profile_tool.test "profile_id:tool_id"
			// Update testing
			{
				Config: testAccProfileToolResourceConfigUpdated(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile_tool.test",
						tfjsonpath.New("profile_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile_tool.test",
						tfjsonpath.New("tool_result_treatment"),
						knownvalue.StringExact("untrusted"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

// Simplified test without import for now - import can be tested manually
func TestAccProfileToolResourceMinimal(t *testing.T) {
	t.Skip("Skipping test - requires actual MCP server and tool setup")
}

func testAccProfileToolResourceConfig() string {
	return `
# Create a test agent (profile)
resource "archestra_agent" "test_profile" {
  name = "test-profile-for-tool-assignment"
}

# Use an existing tool or create a simple MCP server for testing
# For this test, we'll assume there's a tool available in the system
# In a real scenario, you'd need to ensure a tool exists

# If you have a specific MCP server with tools, reference it here
# This is a placeholder - adjust based on your test environment

# For now, let's just test with a basic assignment
resource "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test_profile.id
  # You'll need to replace this with an actual tool ID from your environment
  # Or set up an MCP server installation first
  tool_id    = archestra_agent.test_profile.id  # Placeholder - replace with real tool ID

  use_dynamic_team_credential               = false
  allow_usage_when_untrusted_data_is_present = true
  tool_result_treatment                      = "trusted"
}
`
}

func testAccProfileToolResourceConfigUpdated() string {
	return `
# Create a test agent (profile)
resource "archestra_agent" "test_profile" {
  name = "test-profile-for-tool-assignment"
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test_profile.id
  tool_id    = archestra_agent.test_profile.id  # Placeholder - replace with real tool ID

  use_dynamic_team_credential               = false
  allow_usage_when_untrusted_data_is_present = false
  tool_result_treatment                      = "untrusted"
  response_modifier_template                 = "Modified: {{response}}"
}
`
}
