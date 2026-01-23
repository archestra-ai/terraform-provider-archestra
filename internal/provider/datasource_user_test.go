package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccUserDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserDataSourceConfig("ds-test-user-by-id"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_user.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("ds-test-user-by-id"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_user.test",
						tfjsonpath.New("email"),
						knownvalue.StringExact("ds-test-user-by-id@example.com"),
					),
				},
			},
			{
				Config: testAccUserDataSourceConfigByEmail("ds-test-user-by-email"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_user.test_email",
						tfjsonpath.New("name"),
						knownvalue.StringExact("ds-test-user-by-email"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_user.test_email",
						tfjsonpath.New("email"),
						knownvalue.StringExact("ds-test-user-by-email@example.com"),
					),
				},
			},
		},
	})
}

func testAccUserDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_user" "test" {
  name     = %[1]q
  email    = "%[1]s@example.com"
  password = "password123"
}

data "archestra_user" "test" {
  id = archestra_user.test.id
}
`, name)
}

func testAccUserDataSourceConfigByEmail(name string) string {
	return fmt.Sprintf(`
resource "archestra_user" "test_email" {
  name     = %[1]q
  email    = "%[1]s@example.com"
  password = "password123"
}

data "archestra_user" "test_email" {
  email = archestra_user.test_email.email
}
`, name)
}
