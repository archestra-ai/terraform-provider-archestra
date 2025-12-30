package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccArchestraRole_Basic(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccArchestraRoleConfig(rName, "agents:read"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-role-%s", rName)),
					),
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("permissions"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("agents:read"),
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
				Config: testAccArchestraRoleConfig(rName, "agents:read", "mcp_servers:read"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("test-role-%s", rName)),
					),
					statecheck.ExpectKnownValue(
						"archestra_role.test",
						tfjsonpath.New("permissions"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.StringExact("agents:read"),
							knownvalue.StringExact("mcp_servers:read"),
						}),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccArchestraRoleConfig(rName string, permissions ...string) string {
	perms := ""
	for i, p := range permissions {
		perms += fmt.Sprintf("%q", p)
		if i < len(permissions)-1 {
			perms += ", "
		}
	}
	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name        = "test-role-%[1]s"
  permissions = [%[2]s]
}
`, rName, perms)
}
