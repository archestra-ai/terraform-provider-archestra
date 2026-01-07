package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRoleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccRoleResourceConfig("test-role", `{"agents" = ["read", "create"]}`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-role"),
					),
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("predefined"),
						knownvalue.Bool(false),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_role.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccRoleResourceConfig("test-role-updated", `{"agents" = ["read", "create", "update"], "mcp_servers" = ["read"]}`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-role-updated"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccRoleResourceConfig(name, permissions string) string {
	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name        = %[1]q
  permissions = %[2]s
}
`, name, permissions)
}
