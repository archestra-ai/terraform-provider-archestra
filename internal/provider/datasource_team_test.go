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
			// Create team and read it via data source
			{
				Config: testAccTeamDataSourceConfig("test-team-datasource"),
				ConfigStateChecks: []statecheck.StateCheck{
					// Check that the data source returns the same data as the resource
					statecheck.ExpectKnownValue(
						"data.archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team-datasource"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_team.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Test team for data source testing"),
					),
				},
			},
		},
	})
}

func testAccTeamDataSourceConfig(name string) string {
	return fmt.Sprintf(`
# First create a team
resource "archestra_team" "example" {
  name        = %[1]q
  description = "Test team for data source testing"
}

# Then read it via data source
data "archestra_team" "test" {
  id = archestra_team.example.id
}
`, name)
}

func TestAccTeamDataSource_WithMembers(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamDataSourceConfigWithMembers(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Check that members is populated (may be empty but should exist)
					statecheck.ExpectKnownValue(
						"data.archestra_team.withmembers",
						tfjsonpath.New("members"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_team.withmembers",
						tfjsonpath.New("organization_id"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func testAccTeamDataSourceConfigWithMembers() string {
	return `
resource "archestra_team" "withmembers" {
  name        = "test-team-with-members"
  description = "Team to test members field in data source"
}

data "archestra_team" "withmembers" {
  id = archestra_team.withmembers.id
}
`
}
