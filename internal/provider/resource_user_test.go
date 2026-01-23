package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccUserResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfig("Test User", "abri@example.com", "password123", "http://example.com/image.png"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_user.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("Test User"),
					),
					statecheck.ExpectKnownValue(
						"archestra_user.test",
						tfjsonpath.New("email"),
						knownvalue.StringExact("abri@example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_user.test",
						tfjsonpath.New("image"),
						knownvalue.StringExact("http://example.com/image.png"),
					),
					statecheck.ExpectKnownValue(
						"archestra_user.test",
						tfjsonpath.New("password"),
						knownvalue.StringExact("password123"),
					),
				},
			},
			{
				ResourceName:            "archestra_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
			// Update and Read testing
			{
				Config: testAccUserResourceConfig("Updated User", "updated@example.com", "password123", "http://example.com/updated.png"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_user.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("Updated User"),
					),
					statecheck.ExpectKnownValue(
						"archestra_user.test",
						tfjsonpath.New("email"),
						knownvalue.StringExact("updated@example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_user.test",
						tfjsonpath.New("image"),
						knownvalue.StringExact("http://example.com/updated.png"),
					),
				},
			},
		},
	})
}

func testAccUserResourceConfig(name, email, password, image string) string {
	return fmt.Sprintf(`
resource "archestra_user" "test" {
  name     = %[1]q
  email    = %[2]q
  password = %[3]q
  image    = %[4]q
}
`, name, email, password, image)
}
