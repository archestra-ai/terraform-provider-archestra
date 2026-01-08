package provider

import (
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
			// Create a role first, then look it up
			{
				Config: testAccRoleDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-lookup-role"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_role.test",
						tfjsonpath.New("predefined"),
						knownvalue.Bool(false),
					),
				},
			},
		},
	})
}

func testAccRoleDataSourceConfig() string {
	return `
resource "archestra_role" "created" {
  name = "test-lookup-role"
  permissions = {
    "agents" = ["read"]
  }
}

data "archestra_role" "test" {
  name = archestra_role.created.name
}
`
}
