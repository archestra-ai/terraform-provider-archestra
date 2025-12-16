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
			{
				Config: testAccTeamExternalGroupResourceConfig("engineering-group"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team_external_group.test",
						tfjsonpath.New("external_group_id"),
						knownvalue.StringExact("engineering-group"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team_external_group.test",
						tfjsonpath.New("team_id"),
						knownvalue.StringRegexp(regexp.MustCompile(".+")),
					),
				},
			},
			{
				ResourceName:      "archestra_team_external_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccTeamExternalGroupResourceConfig("engineering-group-updated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team_external_group.test",
						tfjsonpath.New("external_group_id"),
						knownvalue.StringExact("engineering-group-updated"),
					),
				},
			},
		},
	})
}

func testAccTeamExternalGroupResourceConfig(groupID string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name = "tf-team-%[1]s"
}

resource "archestra_team_external_group" "test" {
  team_id           = archestra_team.test.id
  external_group_id = %[1]q
}
`, groupID)
}
