package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserRoleAssignmentResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccUserRoleAssignmentResourceConfig("test-role", "test-user"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("archestra_user_role_assignment.test", "user_id"),
					resource.TestCheckResourceAttrSet("archestra_user_role_assignment.test", "role_id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "archestra_user_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccUserRoleAssignmentResourceConfig(roleName, userName string) string {
	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name        = "%s"
  permissions = ["read:all"]
}

resource "archestra_user" "test" {
  name  = "%s"
  email = "test@example.com"
}

resource "archestra_user_role_assignment" "test" {
  user_id = archestra_user.test.id
  role_id = archestra_role.test.id
}
`, roleName, userName)
}
