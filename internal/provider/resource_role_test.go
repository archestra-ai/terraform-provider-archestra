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
				Config: testAccRoleResourceConfig("test-role", map[string][]string{
					"agents": {"read", "update"},
				}),
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
				Config: testAccRoleResourceConfig("test-role-updated", map[string][]string{
					"agents":      {"read", "update", "create"},
					"mcp_servers": {"read"},
				}),
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

func testAccRoleResourceConfig(name string, permissions map[string][]string) string {
	permsStr := ""
	for resource, actions := range permissions {
		actionsStr := ""
		for i, action := range actions {
			if i > 0 {
				actionsStr += ", "
			}
			actionsStr += fmt.Sprintf("%q", action)
		}
		permsStr += fmt.Sprintf("    %q = [%s]\n", resource, actionsStr)
	}

	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name = %q
  permissions = {
%s  }
}
`, name, permsStr)
}
