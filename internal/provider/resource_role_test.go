package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRoleResource_Basic(t *testing.T) {
	roleName := fmt.Sprintf("acc-role-%s", strings.ToLower(acctest.RandString(6)))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleResourceConfig(roleName, "Administrator role", []string{"read", "write", "delete"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.test", "name", roleName),
					resource.TestCheckResourceAttr("archestra_role.test", "description", "Administrator role"),
					resource.TestCheckResourceAttr("archestra_role.test", "permissions.#", "3"),
				),
			},
			{
				Config: testAccRoleResourceConfig(roleName, "Updated role", []string{"read", "write"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.test", "name", roleName),
					resource.TestCheckResourceAttr("archestra_role.test", "description", "Updated role"),
					resource.TestCheckResourceAttr("archestra_role.test", "permissions.#", "2"),
				),
			},
		},
	})
}

func testAccRoleResourceConfig(name, description string, permissions []string) string {
	config := `
resource "archestra_role" "test" {
  name        = "` + name + `"
  description = "` + description + `"
  permissions = [`

	for i, perm := range permissions {
		if i > 0 {
			config += ", "
		}
		config += `"` + perm + `"`
	}

	config += `]
}
`
	return config
}
