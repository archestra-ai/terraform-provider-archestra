package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTeamResourceConfig("test-team", "Test Description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Test Description"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_team.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccTeamResourceConfig("test-team-updated", "Updated Description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team-updated"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Updated Description"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTeamResourceWithToonCompression(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the team without convert_tool_results_to_toon
			{
				Config: testAccTeamResourceConfig("test-team-toon", "Team for toon compression test"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team-toon"),
					),
				},
			},
			// Step 2: Update the team to enable convert_tool_results_to_toon
			{
				Config: testAccTeamResourceConfigWithToon("test-team-toon", "Team for toon compression test", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team-toon"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("convert_tool_results_to_toon"),
						knownvalue.Bool(true),
					),
				},
			},
		},
	})
}

func testAccTeamResourceConfigWithToon(name, description string, convertToToon bool) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name                         = %[1]q
  description                  = %[2]q
  convert_tool_results_to_toon = %[3]t
}
`, name, description, convertToToon)
}

func testAccTeamResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name        = %[1]q
  description = %[2]q
}
`, name, description)
}
