package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRoleDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create role and read it via data source
			{
				Config: testAccRoleDataSourceConfig("test-role-datasource"),
				ConfigStateChecks: []statecheck.StateCheck{
					// Check that the data source returns the same data as the resource
					statecheck.ExpectKnownValue(
						"data.archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-role-datasource"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_role.test",
						tfjsonpath.New("permissions"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func testAccRoleDataSourceConfig(name string) string {
	return fmt.Sprintf(`
# First create a role
resource "archestra_role" "example" {
  name = %[1]q
  permissions = [
    "agents:read",
    "mcp_servers:write"
  ]
}

# Then read it via data source
data "archestra_role" "test" {
  name = archestra_role.example.name
}
`, name)
}
