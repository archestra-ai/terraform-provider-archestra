package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserRoleAssignment_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserRoleAssignmentConfig("Test User", "roleassign-test@example.com", "password123", "test-assignment-role"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_user.test", "name", "Test User"),
					resource.TestCheckResourceAttr("archestra_user.test", "email", "roleassign-test@example.com"),
					resource.TestCheckResourceAttr("archestra_role.test", "name", "test-assignment-role"),
					resource.TestCheckResourceAttrPair("archestra_user_role_assignment.test", "user_id", "archestra_user.test", "id"),
					resource.TestCheckResourceAttrPair("archestra_user_role_assignment.test", "role_identifier", "archestra_role.test", "name"),
				),
			},
			{
				ResourceName:      "archestra_user_role_assignment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccUserRoleAssignmentConfigUpdated("Test User", "roleassign-test@example.com", "password123", "test-assignment-role", "updated-assignment-role"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.updated", "name", "updated-assignment-role"),
					resource.TestCheckResourceAttrPair("archestra_user_role_assignment.test", "role_identifier", "archestra_role.updated", "name"),
				),
			},
		},
	})
}

func testAccUserRoleAssignmentConfig(userName, email, password, roleName string) string {
	return fmt.Sprintf(`
# First, create a user
resource "archestra_user" "test" {
  name     = %[1]q
  email    = %[2]q
  password = %[3]q
}

# Then, create a role
resource "archestra_role" "test" {
  name        = %[4]q
  permissions = {
    user = ["read"]
  }
}

# Finally, assign the role to the user
resource "archestra_user_role_assignment" "test" {
  user_id         = archestra_user.test.id
  role_identifier = archestra_role.test.name
}
`, userName, email, password, roleName)
}

func testAccUserRoleAssignmentConfigUpdated(userName, email, password, roleName, updatedRoleName string) string {
	return fmt.Sprintf(`
# Keep the user
resource "archestra_user" "test" {
  name     = %[1]q
  email    = %[2]q
  password = %[3]q
}

# Keep the original role (for reference)
resource "archestra_role" "test" {
  name        = %[4]q
  permissions = {
    user = ["read"]
  }
}

# Create a new role to assign
resource "archestra_role" "updated" {
  name        = %[5]q
  permissions = {
    user = ["read", "create"]
  }
}

# Update the assignment to use the new role
resource "archestra_user_role_assignment" "test" {
  user_id         = archestra_user.test.id
  role_identifier = archestra_role.updated.name
}
`, userName, email, password, roleName, updatedRoleName)
}
