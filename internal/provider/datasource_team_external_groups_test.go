package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTeamExternalGroupsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTeamExternalGroupsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_team_external_groups.test", "groups.#"),
					resource.TestCheckResourceAttrSet("data.archestra_team_external_groups.test", "team_id"),
				),
			},
		},
	})
}

func testAccTeamExternalGroupsDataSourceConfig() string {
	return `
resource "archestra_team" "test" {
  name = "tf-ds-team"
}

resource "archestra_team_external_group" "g1" {
  team_id           = archestra_team.test.id
  external_group_id = "ext-1"
}

resource "archestra_team_external_group" "g2" {
  team_id           = archestra_team.test.id
  external_group_id = "ext-2"
}

data "archestra_team_external_groups" "test" {
  team_id = archestra_team.test.id
}
`
}
