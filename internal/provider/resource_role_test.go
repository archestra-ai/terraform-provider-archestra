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
	roleName := "Test Role"
	updatedRoleName := "Updated Test Role"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccRoleResourceConfig(roleName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(roleName),
					),
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("permission"),
						knownvalue.SetPartial([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"resource": knownvalue.StringExact("agent"),
								"actions": knownvalue.SetExact([]knownvalue.Check{
									knownvalue.StringExact("read"),
									knownvalue.StringExact("update"),
								}),
							}),
						}),
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
				Config: testAccRoleResourceConfig(updatedRoleName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(updatedRoleName),
					),
				},
			},
		},
	})
}

func testAccRoleResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name = %[1]q
  permission = [
    {
      resource = "agent"
      actions  = ["read", "update"]
    }
  ]
}
`, name)
}
