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

func TestAccOrganizationSettingsResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccOrganizationSettingsResourceConfig("lato", "amber-minimal", "dummy-base64-logo", "1h", "organization", "true", "false"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("font"),
						knownvalue.StringExact("lato"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("color_theme"),
						knownvalue.StringExact("amber-minimal"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("logo"),
						knownvalue.StringExact("dummy-base64-logo"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("limit_cleanup_interval"),
						knownvalue.StringExact("1h"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("compression_scope"),
						knownvalue.StringExact("organization"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("onboarding_complete"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("convert_tool_results_to_toon"),
						knownvalue.Bool(false),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_organization_settings.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccOrganizationSettingsResourceConfig("roboto", "midnight-bloom", "new-dummy-logo", "24h", "team", "false", "true"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("font"),
						knownvalue.StringExact("roboto"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("color_theme"),
						knownvalue.StringExact("midnight-bloom"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("logo"),
						knownvalue.StringExact("new-dummy-logo"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("limit_cleanup_interval"),
						knownvalue.StringExact("24h"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("compression_scope"),
						knownvalue.StringExact("team"),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("onboarding_complete"),
						knownvalue.Bool(false),
					),
					statecheck.ExpectKnownValue(
						"archestra_organization_settings.test",
						tfjsonpath.New("convert_tool_results_to_toon"),
						knownvalue.Bool(true),
					),
				},
			},
		},
	})
}

func TestAccOrganizationSettingsResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOrganizationSettingsResourceConfig("comic-sans", "amber-minimal", "", "1h", "organization", "", ""),
				ExpectError: regexp.MustCompile(`Attribute font value must be one of:`),
			},
			{
				Config:      testAccOrganizationSettingsResourceConfig("lato", "matrix", "", "1h", "organization", "", ""),
				ExpectError: regexp.MustCompile(`Attribute color_theme value must be one of:`),
			},
			{
				Config:      testAccOrganizationSettingsResourceConfig("lato", "amber-minimal", "", "2h", "organization", "", ""),
				ExpectError: regexp.MustCompile(`Attribute limit_cleanup_interval value must be one of:`),
			},
			{
				Config:      testAccOrganizationSettingsResourceConfig("lato", "amber-minimal", "", "1h", "global", "", ""),
				ExpectError: regexp.MustCompile(`Attribute compression_scope value must be one of:`),
			},
		},
	})
}

func testAccOrganizationSettingsResourceConfig(font, theme, logo, interval, scope, onboarding, convert string) string {
	logoLine := ""
	if logo != "" {
		logoLine = fmt.Sprintf(`logo = "%s"`, logo)
	}
	intervalLine := ""
	if interval != "" {
		intervalLine = fmt.Sprintf(`limit_cleanup_interval = "%s"`, interval)
	}
	scopeLine := ""
	if scope != "" {
		scopeLine = fmt.Sprintf(`compression_scope = "%s"`, scope)
	}
	onboardingLine := ""
	if onboarding != "" {
		onboardingLine = fmt.Sprintf(`onboarding_complete = %s`, onboarding)
	}
	convertLine := ""
	if convert != "" {
		convertLine = fmt.Sprintf(`convert_tool_results_to_toon = %s`, convert)
	}
	return fmt.Sprintf(`
resource "archestra_organization_settings" "test" {
  font        = %[1]q
  color_theme = %[2]q
  %[3]s
  %[4]s
  %[5]s
  %[6]s
  %[7]s
}
`, font, theme, logoLine, intervalLine, scopeLine, onboardingLine, convertLine)
}
