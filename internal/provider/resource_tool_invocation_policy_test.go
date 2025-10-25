package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccToolInvocationPolicyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccToolInvocationPolicyResourceConfig("agent-tool-id", "path", "contains", "/etc/", "block_always", "Cannot write to system directories"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("argument_name"),
						knownvalue.StringExact("path"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("contains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("/etc/"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_tool_invocation_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccToolInvocationPolicyResourceConfig("agent-tool-id", "path", "startsWith", "/home/", "allow_when_context_is_untrusted", "Allow home directory access when context is untrusted"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("startsWith"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("/home/"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("allow_when_context_is_untrusted"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccToolInvocationPolicyResourceConfig(agentToolID, argumentName, operator, value, action, description string) string {
	return fmt.Sprintf(`
resource "archestra_tool_invocation_policy" "test" {
  agent_tool_id = %[1]q
  argument_name = %[2]q
  operator      = %[3]q
  value         = %[4]q
  action        = %[5]q
  description   = %[6]q
}
`, agentToolID, argumentName, operator, value, action, description)
}
