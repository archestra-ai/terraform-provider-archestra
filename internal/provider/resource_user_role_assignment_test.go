package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const testAccUserEnvVar = "ARCHES_TEST_USER_ID"

func TestAccUserRoleAssignmentResource_Basic(t *testing.T) {
	userID := os.Getenv(testAccUserEnvVar)
	if userID == "" {
		t.Skipf("set %s to run user role assignment acceptance test", testAccUserEnvVar)
	}

	roleName := fmt.Sprintf("acc-role-%s", strings.ToLower(acctest.RandString(6)))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserRoleAssignmentResourceConfig(userID, roleName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_user_role_assignment.test", "user_id", userID),
					resource.TestCheckResourceAttrSet("archestra_user_role_assignment.test", "id"),
				),
			},
		},
	})
}

func testAccUserRoleAssignmentResourceConfig(userID, roleName string) string {
	return `
resource "archestra_role" "role" {
  name        = "` + roleName + `"
  description = "assignment role"
  permissions = ["read"]
}

resource "archestra_user_role_assignment" "test" {
  user_id = "` + userID + `"
  role_id = archestra_role.role.id
}
`
}
