package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTrustedDataPolicyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTrustedDataPolicyResourceConfig("agent-tool-id", "url", "contains", "api.company.com", "mark_as_trusted", "Trust company API"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("attribute_path"),
						knownvalue.StringExact("url"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("contains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("api.company.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("mark_as_trusted"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_trusted_data_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccTrustedDataPolicyResourceConfig("agent-tool-id", "url", "regex", "^https://verified\\.company\\.com/.*$", "mark_as_trusted", "Trust verified company API"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("regex"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("^https://verified\\.company\\.com/.*$"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccTrustedDataPolicyResourceConfig(agentToolID, attributePath, operator, value, action, description string) string {
	return fmt.Sprintf(`
resource "archestra_trusted_data_policy" "test" {
  agent_tool_id  = %[1]q
  attribute_path = %[2]q
  operator       = %[3]q
  value          = %[4]q
  action         = %[5]q
  description    = %[6]q
}
`, agentToolID, attributePath, operator, value, action, description)
}
