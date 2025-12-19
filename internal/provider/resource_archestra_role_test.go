package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccArchestraRole_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: testAccArchestraRoleConfig("Developer", "agents:read"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.test", "name", "Developer"),
					resource.TestCheckResourceAttr("archestra_role.test", "permissions.#", "1"),
					resource.TestCheckResourceAttr("archestra_role.test", "permissions.0", "agents:read"),
				),
			},
			// Update
			{
				Config: testAccArchestraRoleConfig("Developer", "agents:read", "mcp_servers:read"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.test", "name", "Developer"),
					resource.TestCheckResourceAttr("archestra_role.test", "permissions.#", "2"),
				),
			},
			// Import
			{
				ResourceName:      "archestra_role.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccArchestraRoleConfig(name string, permissions ...string) string {
	perms := ""
	for i, p := range permissions {
		perms += fmt.Sprintf("\"%s\"", p)
		if i < len(permissions)-1 {
			perms += ", "
		}
	}
	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name        = "%s"
  permissions = [%s]
}
`, name, perms)
}
