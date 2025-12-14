package provider

import (
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
			{
				Config: testAccSSOProviderConfig,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.test",
						tfjsonpath.New("provider_id"),
						knownvalue.StringExact("google"),
					),
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.test",
						tfjsonpath.New("issuer"),
						knownvalue.StringExact("https://accounts.google.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_sso_provider.test",
						tfjsonpath.New("domain"),
						knownvalue.StringExact("example.com"),
					),
				},
			},
		},
	})
}

const testAccSSOProviderConfig = `
resource "archestra_sso_provider" "test" {
  provider_id = "google"
  issuer      = "https://accounts.google.com"
  domain      = "example.com"
}
`
