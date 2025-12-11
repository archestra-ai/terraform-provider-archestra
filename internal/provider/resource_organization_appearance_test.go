package provider

import (
	"fmt"
	"regexp"
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
				Config: testAccOrganizationAppearanceResourceConfig("lato", "amber-minimal", "dummy-base64-logo"),
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
						tfjsonpath.New("logo"),
						knownvalue.StringExact("dummy-base64-logo"),
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
				Config: testAccOrganizationAppearanceResourceConfig("roboto", "midnight-bloom", "new-dummy-logo"),
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
						tfjsonpath.New("logo"),
						knownvalue.StringExact("new-dummy-logo"),
					),
				},
			},
		},
	})
}

func TestAccOrganizationAppearanceResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOrganizationAppearanceResourceConfig("comic-sans", "amber-minimal", ""),
				ExpectError: regexp.MustCompile(`Attribute font value must be one of:`),
			},
			{
				Config:      testAccOrganizationAppearanceResourceConfig("lato", "matrix", ""),
				ExpectError: regexp.MustCompile(`Attribute color_theme value must be one of:`),
			},
		},
	})
}

func testAccOrganizationAppearanceResourceConfig(font, theme, logo string) string {
	logoLine := ""
	if logo != "" {
		logoLine = fmt.Sprintf(`logo = "%s"`, logo)
	}
	return fmt.Sprintf(`
resource "archestra_organization_appearance" "test" {
  font        = %[1]q
  color_theme = %[2]q
  %[3]s
}
`, font, theme, logoLine)
}
