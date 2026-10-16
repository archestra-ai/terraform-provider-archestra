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
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "onboarding_complete", "true"),
					resource.TestCheckResourceAttrSet("archestra_organization_settings.test", "id"),
					// Read-only org-identity fields. The backend declares
					// name/slug/created_at as NOT NULL so these must
					// populate after every Read; if a future refactor drops
					// the Read mapping, this catches it.
					resource.TestCheckResourceAttrSet("archestra_organization_settings.test", "name"),
					resource.TestCheckResourceAttrSet("archestra_organization_settings.test", "slug"),
					resource.TestCheckResourceAttrSet("archestra_organization_settings.test", "created_at"),
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
				),
			},
		},
	})
}

func TestAccOrganizationSettingsResourceRejectsRetiredCleanup(t *testing.T) {
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: testAccOrganizationSettingsResourceConfigWithCleanup("24h"), ExpectError: regexp.MustCompile("Unsupported argument")}}})
}

func testAccOrganizationSettingsResourceConfig(font, theme, _ string, onboarding, _ bool) string {
	onboardingStr := "false"
	if onboarding {
		onboardingStr = "true"
	}

	return `
resource "archestra_organization_settings" "test" {
  font                         = "` + font + `"
  color_theme                  = "` + theme + `"
  onboarding_complete          = ` + onboardingStr + `
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
				ExpectError: regexp.MustCompile(`Unsupported argument`),
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
				ExpectError: regexp.MustCompile(`Unsupported argument`),
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

func TestAccOrganizationSettingsResourceWithAppearance(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationSettingsResourceConfigWithAppearance("Test App", "restrictive", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "app_name", "Test App"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "allow_chat_file_uploads", "false"),
					resource.TestCheckResourceAttrSet("archestra_organization_settings.test", "id"),
				),
			},
			{
				Config: testAccOrganizationSettingsResourceConfigWithAppearance("Updated App", "permissive", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "app_name", "Updated App"),
					resource.TestCheckResourceAttr("archestra_organization_settings.test", "allow_chat_file_uploads", "true"),
				),
			},
		},
	})
}

func testAccOrganizationSettingsResourceConfigWithAppearance(appName, _ string, allowUploads bool) string {
	allowUploadsStr := "false"
	if allowUploads {
		allowUploadsStr = "true"
	}

	return `
resource "archestra_organization_settings" "test" {
  onboarding_complete      = true
  app_name                 = "` + appName + `"
  allow_chat_file_uploads  = ` + allowUploadsStr + `
}
`
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
