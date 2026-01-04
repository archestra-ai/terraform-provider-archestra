package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRoleDataSource_Basic(t *testing.T) {
	roleName := fmt.Sprintf("acc-ds-role-%s", strings.ToLower(acctest.RandString(6)))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleDataSourceConfig(roleName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.archestra_role.test", "name", roleName),
					resource.TestCheckResourceAttr("data.archestra_role.test", "permissions.#", "2"),
				),
			},
		},
	})
}

func testAccRoleDataSourceConfig(name string) string {
	return `
resource "archestra_role" "setup" {
  name        = "` + name + `"
  description = "data source role"
  permissions = ["read", "write"]
}

data "archestra_role" "test" {
  name = archestra_role.setup.name
}
`
}
