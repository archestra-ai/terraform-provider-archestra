package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccProfileDataSource(t *testing.T) {
	t.Skip("Skipping test - would require actual Archestra backend with agents")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccProfileDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_profile.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-profile-datasource"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_profile.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func testAccProfileDataSourceConfig() string {
	return `
# Create a test agent to look up
resource "archestra_agent" "test" {
  name = "test-profile-datasource"
}

# Look up the profile by name
data "archestra_profile" "test" {
  name = archestra_agent.test.name
}
`
}
