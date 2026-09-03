package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRuntimeCredentialResource(t *testing.T) {
	key := acctest.RandomWithPrefix("tf-acc-cred")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRuntimeCredentialConfig(key, "GitHub token", "A token."),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_runtime_credential.test", tfjsonpath.New("id"), knownvalue.StringExact(key)),
					statecheck.ExpectKnownValue("archestra_runtime_credential.test", tfjsonpath.New("allow_personal"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("archestra_runtime_credential.test", tfjsonpath.New("allow_organization"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("archestra_runtime_credential.test", tfjsonpath.New("organization_configured"), knownvalue.Bool(false)),
				},
			},
			{
				ResourceName:      "archestra_runtime_credential.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// PATCH-able field update — must not force replacement (asserted
			// via the id staying importable and the plan applying in place).
			{
				Config: testAccRuntimeCredentialConfig(key, "GitHub token", "An updated token description."),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_runtime_credential.test", tfjsonpath.New("description"), knownvalue.StringExact("An updated token description.")),
				},
			},
		},
	})
}

func TestAccRuntimeCredentialResource_OrganizationValue(t *testing.T) {
	key := acctest.RandomWithPrefix("tf-acc-org-cred")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRuntimeCredentialOrgConfig(key, "secret-one"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_runtime_credential.org", tfjsonpath.New("allow_organization"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("archestra_runtime_credential.org", tfjsonpath.New("organization_configured"), knownvalue.Bool(true)),
				},
			},
			// Rotate the value in place.
			{
				Config: testAccRuntimeCredentialOrgConfig(key, "secret-two"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_runtime_credential.org", tfjsonpath.New("organization_configured"), knownvalue.Bool(true)),
				},
			},
			// Removing the attribute disconnects the stored value but keeps
			// the definition.
			{
				Config: testAccRuntimeCredentialOrgConfigNoValue(key),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_runtime_credential.org", tfjsonpath.New("organization_configured"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func testAccRuntimeCredentialConfig(key, name, description string) string {
	return fmt.Sprintf(`
resource "archestra_runtime_credential" "test" {
  key         = %q
  name        = %q
  description = %q
}
`, key, name, description)
}

func testAccRuntimeCredentialOrgConfig(key, value string) string {
	return fmt.Sprintf(`
resource "archestra_runtime_credential" "org" {
  key                = %q
  name               = "Org credential"
  allow_personal     = false
  allow_organization = true
  organization_value = %q
}
`, key, value)
}

func testAccRuntimeCredentialOrgConfigNoValue(key string) string {
	return fmt.Sprintf(`
resource "archestra_runtime_credential" "org" {
  key                = %q
  name               = "Org credential"
  allow_personal     = false
  allow_organization = true
}
`, key)
}
