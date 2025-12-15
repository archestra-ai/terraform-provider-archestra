package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRoleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccRoleResourceConfig("test-role", "Test Role Description", "read:stuff"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.test", "name", "test-role"),
					resource.TestCheckResourceAttr("archestra_role.test", "description", "Test Role Description"),
					resource.TestCheckResourceAttr("archestra_role.test", "permissions.0", "read:stuff"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "archestra_role.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccRoleResourceConfig("test-role-updated", "Updated Description", "write:stuff"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.test", "name", "test-role-updated"),
					resource.TestCheckResourceAttr("archestra_role.test", "description", "Updated Description"),
					resource.TestCheckResourceAttr("archestra_role.test", "permissions.0", "write:stuff"),
				),
			},
		},
	})
}

func testAccRoleResourceConfig(name, description, permission string) string {
	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name        = "%s"
  description = "%s"
  permissions = ["%s"]
}
`, name, description, permission)
}
