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
data "archestra_team_external_groups" "test" {
  team_id = "test-team-id"
}
`
}
