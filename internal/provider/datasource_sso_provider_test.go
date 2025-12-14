package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSSOProviderDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSSOProviderDataSourceConfig,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_sso_provider.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_sso_provider.test",
						tfjsonpath.New("provider_id"),
						knownvalue.StringExact("google"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_sso_provider.test",
						tfjsonpath.New("issuer"),
						knownvalue.StringExact("https://accounts.google.com"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_sso_provider.test",
						tfjsonpath.New("domain"),
						knownvalue.StringExact("example.com"),
					),
				},
			},
		},
	})
}

const testAccSSOProviderDataSourceConfig = `
data "archestra_sso_provider" "test" {
  id = "test-sso-provider-id"
}
`
