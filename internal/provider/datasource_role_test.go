package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRoleDataSource(t *testing.T) {
	// TODO: Enable when Role API backend is implemented (currently returns 500)
	t.Skip("Skipping: Role API returns 500 - backend implementation pending")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a role first, then look it up by name
			{
				Config: testAccRoleDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-role-datasource"),
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

func TestAccRoleDataSourceByID(t *testing.T) {
	// TODO: Enable when Role API backend is implemented (currently returns 500)
	t.Skip("Skipping: Role API returns 500 - backend implementation pending")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a role first, then look it up by ID
			{
				Config: testAccRoleDataSourceByIDConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_role.by_id",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-role-lookup-by-id"),
					),
				},
			},
		},
	})
}

func testAccRoleDataSourceConfig() string {
	return `
resource "archestra_role" "test" {
  name = "test-role-datasource"
  permissions = {
    "agents" = ["read"]
  }
}

data "archestra_role" "test" {
  name = archestra_role.test.name
}
`
}

func testAccRoleDataSourceByIDConfig() string {
	return `
resource "archestra_role" "test" {
  name = "test-role-lookup-by-id"
  permissions = {
    "agents" = ["read", "create"]
  }
}

data "archestra_role" "by_id" {
  id = archestra_role.test.id
}
`
}
