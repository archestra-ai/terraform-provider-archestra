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
			{
				Config: testAccRoleResourceConfig("test-role1", "Test Description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-role1"),
					),
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("permissions"),
						knownvalue.MapExact(map[string]knownvalue.Check{
							"user": knownvalue.ListExact([]knownvalue.Check{
								knownvalue.StringExact("read"),
								knownvalue.StringExact("create"),
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
			// // Update and Read testing
			{
				Config: testAccRoleResourceConfig("test-role-updated", "Updated Description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-role-updated"),
					),
				},
			},
		},
	})
}

func testAccRoleResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name        = %[1]q
  permissions = {
    user = ["read", "create"]
  }
}
`, name, description)
}
