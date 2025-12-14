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

func TestAccOrganizationSettings(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create and Read testing
			{
				Config: testAccOrganizationSettingsConfig("roboto", "ocean-breeze", "initial-logo-base64"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.main",
						tfjsonpath.New("font"),
						knownvalue.StringExact("roboto"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.main",
						tfjsonpath.New("color_theme"),
						knownvalue.StringExact("ocean-breeze"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.main",
						tfjsonpath.New("logo"),
						knownvalue.StringExact("initial-logo-base64"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.main",
						tfjsonpath.New("compression_scope"),
						knownvalue.StringExact("organization"),
					),
				},
			},
			// Step 2: ImportState testing
			{
				ResourceName:      "archestra_organization_settings.main",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update and Read testing
			{
				Config: testAccOrganizationSettingsConfig("inter", "amber-minimal", "updated-logo-base64"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.main",
						tfjsonpath.New("font"),
						knownvalue.StringExact("inter"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.main",
						tfjsonpath.New("color_theme"),
						knownvalue.StringExact("amber-minimal"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.main",
						tfjsonpath.New("logo"),
						knownvalue.StringExact("updated-logo-base64"),
					),
				},
			},
		},
	})
}

// validation tests: Ensure invalid values trigger errors
func TestAccOrganizationSettings_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOrganizationSettingsConfig("comic-sans", "ocean-breeze", ""),
				ExpectError: regexp.MustCompile(`Attribute font value must be one of:`),
			},
			{
				Config:      testAccOrganizationSettingsConfig("roboto", "ugly-theme", ""),
				ExpectError: regexp.MustCompile(`Attribute color_theme value must be one of:`),
			},
			{
				Config: `
resource "archestra_organization_settings" "main" {
  limit_cleanup_interval = "99h"
}
`,
				ExpectError: regexp.MustCompile(`Attribute limit_cleanup_interval value must be one of:`),
			},
		},
	})
}

func testAccOrganizationSettingsConfig(font, theme, logo string) string {
	// We handle empty logo logic here to keep the config cleaner
	logoLine := ""
	if logo != "" {
		logoLine = fmt.Sprintf(`logo = "%s"`, logo)
	}

	return fmt.Sprintf(`
resource "archestra_organization_settings" "main" {
  font                   = "%s"
  color_theme            = "%s"
  limit_cleanup_interval = "1w"
  compression_scope      = "organization"
  onboarding_complete    = true
  %s
}
`, font, theme, logoLine)
}
