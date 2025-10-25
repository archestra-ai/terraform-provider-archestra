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
				Config: testAccTeamResourceConfig("test-team", "Test Description", "org-123", "user-456"),
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
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("organization_id"),
						knownvalue.StringExact("org-123"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("created_by"),
						knownvalue.StringExact("user-456"),
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
				Config: testAccTeamResourceConfig("test-team-updated", "Updated Description", "org-123", "user-456"),
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

func testAccTeamResourceConfig(name, description, orgID, createdBy string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name            = %[1]q
  description     = %[2]q
  organization_id = %[3]q
  created_by      = %[4]q
}
`, name, description, orgID, createdBy)
}
