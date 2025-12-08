package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOrganizationAppearanceResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccOrganizationAppearanceResourceConfig("lato", "amber-minimal", "custom"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_organization_appearance.test",
						tfjsonpath.New("font"),
						knownvalue.StringExact("lato"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_appearance.test",
						tfjsonpath.New("color_theme"),
						knownvalue.StringExact("amber-minimal"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_appearance.test",
						tfjsonpath.New("logo_type"),
						knownvalue.StringExact("custom"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_organization_appearance.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccOrganizationAppearanceResourceConfig("roboto", "midnight-bloom", "default"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_organization_appearance.test",
						tfjsonpath.New("font"),
						knownvalue.StringExact("roboto"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_appearance.test",
						tfjsonpath.New("color_theme"),
						knownvalue.StringExact("midnight-bloom"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_appearance.test",
						tfjsonpath.New("logo_type"),
						knownvalue.StringExact("default"),
					),
				},
			},
		},
	})
}

func testAccOrganizationAppearanceResourceConfig(font, theme, logoType string) string {
	return fmt.Sprintf(`
resource "archestra_organization_appearance" "test" {
  font        = %[1]q
  color_theme = %[2]q
  logo_type   = %[3]q
}
`, font, theme, logoType)
}
