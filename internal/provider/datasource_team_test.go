package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTeamDataSourceConfig("team-id-here"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_team.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("team-id-here"),
					),
				},
			},
		},
	})
}

func testAccTeamDataSourceConfig(teamID string) string {
	return fmt.Sprintf(`
data "archestra_team" "test" {
  id = %[1]q
}
`, teamID)
}
