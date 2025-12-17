package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationSettingsResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationSettingsResourceConfig("inter", "modern-minimal", "organization", true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "font", "inter"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "color_theme", "modern-minimal"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "compression_scope", "organization"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "onboarding_complete", "true"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "convert_tool_results_to_toon", "false"),
					resource.TestCheckResourceAttrSet("archestra_organization_settings.test", "id"),
				),
			},
			{
				ResourceName:      "archestra_organization_settings.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccOrganizationSettingsResourceConfig("roboto", "claude", "team", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "font", "roboto"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "color_theme", "claude"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "compression_scope", "team"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "convert_tool_results_to_toon", "true"),
				),
			},
		},
	})
}

func TestAccOrganizationSettingsResourceWithLimitCleanup(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationSettingsResourceConfigWithCleanup("24h"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "limit_cleanup_interval", "24h"),
					resource.TestCheckResourceAttrSet("archestra_organization_settings.test", "id"),
				),
			},
			{
				Config: testAccOrganizationSettingsResourceConfigWithCleanup("1w"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "limit_cleanup_interval", "1w"),
				),
			},
		},
	})
}

func testAccOrganizationSettingsResourceConfig(font, theme, scope string, onboarding, convert bool) string {
	onboardingStr := "false"
	if onboarding {
		onboardingStr = "true"
	}
	convertStr := "false"
	if convert {
		convertStr = "true"
	}

	return `
resource "archestra_organization_settings" "test" {
  font                         = "` + font + `"
  color_theme                  = "` + theme + `"
  compression_scope            = "` + scope + `"
  onboarding_complete          = ` + onboardingStr + `
  convert_tool_results_to_toon = ` + convertStr + `
}
`
}

func testAccOrganizationSettingsResourceConfigWithCleanup(interval string) string {
	return `
resource "archestra_organization_settings" "test" {
  onboarding_complete    = true
  limit_cleanup_interval = "` + interval + `"
}
`
}

func TestAccOrganizationSettingsResourceInvalidFont(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOrganizationSettingsResourceConfigInvalidFont(),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestAccOrganizationSettingsResourceInvalidTheme(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOrganizationSettingsResourceConfigInvalidTheme(),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestAccOrganizationSettingsResourceInvalidCompressionScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOrganizationSettingsResourceConfigInvalidCompressionScope(),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestAccOrganizationSettingsResourceInvalidLimitCleanupInterval(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccOrganizationSettingsResourceConfigInvalidLimitCleanupInterval(),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestAccOrganizationSettingsResourceWithLogo(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationSettingsResourceConfigWithLogo(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "logo", "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="),
					resource.TestCheckResourceAttrSet("archestra_organization_settings.test", "id"),
				),
			},
			{
				Config: testAccOrganizationSettingsResourceConfigWithLogoUpdated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "logo", "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="),
				),
			},
		},
	})
}

func testAccOrganizationSettingsResourceConfigInvalidFont() string {
	return `
resource "archestra_organization_settings" "test" {
  font = "invalid-font"
}
`
}

func testAccOrganizationSettingsResourceConfigInvalidTheme() string {
	return `
resource "archestra_organization_settings" "test" {
  color_theme = "invalid-theme"
}
`
}

func testAccOrganizationSettingsResourceConfigInvalidCompressionScope() string {
	return `
resource "archestra_organization_settings" "test" {
  compression_scope = "invalid-scope"
}
`
}

func testAccOrganizationSettingsResourceConfigInvalidLimitCleanupInterval() string {
	return `
resource "archestra_organization_settings" "test" {
  limit_cleanup_interval = "invalid-interval"
}
`
}

func testAccOrganizationSettingsResourceConfigWithLogo() string {
	return `
resource "archestra_organization_settings" "test" {
  onboarding_complete = true
  logo                = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
}
`
}

func testAccOrganizationSettingsResourceConfigWithLogoUpdated() string {
	return `
resource "archestra_organization_settings" "test" {
  onboarding_complete = true
  logo                = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
}
`
}
