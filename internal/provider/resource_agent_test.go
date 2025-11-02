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
				Config: testAccAgentResourceConfig("test-agent"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-agent"),
					),
					// Verify labels are in the correct order (alphabetical by key)
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("environment"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("value"),
						knownvalue.StringExact("test"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("labels").AtSliceIndex(1).AtMapKey("key"),
						knownvalue.StringExact("team"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("labels").AtSliceIndex(1).AtMapKey("value"),
						knownvalue.StringExact("engineering"),
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
				Config: testAccAgentResourceConfigUpdated("test-agent-updated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-agent-updated"),
					),
					// Verify label order is preserved after update
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("environment"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("value"),
						knownvalue.StringExact("production"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("labels").AtSliceIndex(1).AtMapKey("key"),
						knownvalue.StringExact("region"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent.test",
						tfjsonpath.New("labels").AtSliceIndex(1).AtMapKey("value"),
						knownvalue.StringExact("us-west-2"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccAgentResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name = %[1]q

  labels = [
    {
      key   = "team"
      value = "engineering"
    },
    {
      key   = "environment"
      value = "test"
    }
  ]
}
`, name)
}

func testAccAgentResourceConfigUpdated(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name = %[1]q

  labels = [
    {
      key   = "environment"
      value = "production"
    },
    {
      key   = "region"
      value = "us-west-2"
    }
  ]
}
`, name)
}
