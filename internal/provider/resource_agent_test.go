package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAgentResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAgentResourceConfig("test-agent", false, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-agent"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("is_demo"),
						knownvalue.Bool(false),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("is_default"),
						knownvalue.Bool(false),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_agent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccAgentResourceConfig("test-agent-updated", true, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-agent-updated"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("is_demo"),
						knownvalue.Bool(true),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccAgentResourceConfig(name string, isDemo bool, isDefault bool) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name       = %[1]q
  is_demo    = %[2]t
  is_default = %[3]t
}
`, name, isDemo, isDefault)
}
