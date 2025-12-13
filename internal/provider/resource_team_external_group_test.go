package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamExternalGroupResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTeamExternalGroupResourceConfig("test-external-group"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team_external_group.test",
						tfjsonpath.New("external_group_id"),
						knownvalue.StringExact("test-external-group"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team_external_group.test",
						tfjsonpath.New("team_id"),
						knownvalue.StringRegexp(regexp.MustCompile(".*")), // Any non-empty team_id
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_team_external_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccTeamExternalGroupResourceConfigUpdated("updated-external-group"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team_external_group.test",
						tfjsonpath.New("external_group_id"),
						knownvalue.StringExact("updated-external-group"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccTeamExternalGroupResourceConfig(externalGroupID string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name = "tf-team-%[1]s"
}

resource "archestra_team_external_group" "test" {
  team_id           = archestra_team.test.id
  external_group_id = %[1]q
}
`, externalGroupID)
}

func testAccTeamExternalGroupResourceConfigUpdated(externalGroupID string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name = "tf-team-updated-%[1]s"
}

resource "archestra_team_external_group" "test" {
  team_id           = archestra_team.test.id
  external_group_id = %[1]q
}
`, externalGroupID)
}

func TestAccTeamExternalGroupResource_WithExternalName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with external_group_name instead of external_group_id
			{
				Config: testAccTeamExternalGroupResourceConfigWithName("test-group-name"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team_external_group.byname",
						tfjsonpath.New("external_group_name"),
						knownvalue.StringExact("test-group-name"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_team_external_group.byname",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update with different name
			{
				Config: testAccTeamExternalGroupResourceConfigWithName("updated-group-name"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team_external_group.byname",
						tfjsonpath.New("external_group_name"),
						knownvalue.StringExact("updated-group-name"),
					),
				},
			},
		},
	})
}

func testAccTeamExternalGroupResourceConfigWithName(externalGroupName string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name = "tf-team-byname-%[1]s"
}

resource "archestra_team_external_group" "byname" {
  team_id              = archestra_team.test.id
  external_group_name  = %[1]q
}
`, externalGroupName)
}
