package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSSOProviderResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing (OIDC)
			{
				Config: testAccSSOProviderResourceConfigOIDC("okta-oidc", "okta.com", "client-id", "secret"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.oidc_test",
						tfjsonpath.New("provider_id"),
						knownvalue.StringExact("okta-oidc"),
					),
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.oidc_test",
						tfjsonpath.New("domain"),
						knownvalue.StringExact("okta.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.oidc_test",
						tfjsonpath.New("oidc_config").AtMapKey("client_id"),
						knownvalue.StringExact("client-id"),
					),
				},
			},
			// Update and Read testing (Update client secret)
			{
				Config: testAccSSOProviderResourceConfigOIDC("okta-oidc", "okta.com", "client-id", "secret-v2"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.oidc_test",
						tfjsonpath.New("oidc_config").AtMapKey("client_secret"),
						knownvalue.StringExact("secret-v2"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_sso_provider.oidc_test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Create SAML provider
			{
				Config: testAccSSOProviderResourceConfigSAML("okta-saml", "okta.com", "https://okta.com/sso", "cert-data", "https://callback.com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.saml_test",
						tfjsonpath.New("provider_id"),
						knownvalue.StringExact("okta-saml"),
					),
				},
			},
		},
	})
}

func testAccSSOProviderResourceConfigOIDC(providerId, domain, clientId, clientSecret string) string {
	return fmt.Sprintf(`
resource "archestra_sso_provider" "oidc_test" {
  provider_id = %[1]q
  domain      = %[2]q

  oidc_config = {
    client_id          = %[3]q
    client_secret      = %[4]q
    discovery_endpoint = "https://accounts.google.com/.well-known/openid-configuration"
    issuer             = "https://accounts.google.com"
  }
}
`, providerId, domain, clientId, clientSecret)
}

func testAccSSOProviderResourceConfigSAML(providerId, domain, entryPoint, cert, callbackUrl string) string {
	return fmt.Sprintf(`
resource "archestra_sso_provider" "saml_test" {
  provider_id = %[1]q
  domain      = %[2]q

  saml_config = {
    entry_point  = %[3]q
    cert         = %[4]q
    callback_url = %[5]q
  }
}
`, providerId, domain, entryPoint, cert, callbackUrl)
}
