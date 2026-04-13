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

func TestAccProfileResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccProfileResourceConfig("test-profile"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-profile"),
					),
					// Verify labels are in configuration order
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("team"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("value"),
						knownvalue.StringExact("engineering"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(1).AtMapKey("key"),
						knownvalue.StringExact("environment"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(1).AtMapKey("value"),
						knownvalue.StringExact("test"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_profile.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Ignore labels during import verification since API returns them in different order
				ImportStateVerifyIgnore: []string{"labels"},
			},
			// Update and Read testing
			{
				Config: testAccProfileResourceConfigUpdated("test-profile-updated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-profile-updated"),
					),
					// Verify label order is preserved after update
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("environment"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("value"),
						knownvalue.StringExact("production"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(1).AtMapKey("key"),
						knownvalue.StringExact("region"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(1).AtMapKey("value"),
						knownvalue.StringExact("us-west-2"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccProfileResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = %[1]q

  labels = [
    {
      key   = "team"
      value = "engineering"
    },
    {
      key   = "environment"
      value = "test"
    }
  ]
}
`, name)
}

func testAccProfileResourceConfigUpdated(name string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = %[1]q

  labels = [
    {
      key   = "environment"
      value = "production"
    },
    {
      key   = "region"
      value = "us-west-2"
    }
  ]
}
`, name)
}

func TestAccProfileResource_WithoutLabels(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create profile without labels
			{
				Config: testAccProfileResourceConfigNoLabels("test-profile-no-labels"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile.nolabels",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-profile-no-labels"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_profile.nolabels",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update to add labels
			{
				Config: testAccProfileResourceConfigAddLabels("test-profile-no-labels"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile.nolabels",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("added"),
					),
				},
			},
		},
	})
}

func testAccProfileResourceConfigNoLabels(name string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "nolabels" {
  name = %[1]q
}
`, name)
}

func testAccProfileResourceConfigAddLabels(name string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "nolabels" {
  name = %[1]q

  labels = [
    {
      key   = "added"
      value = "later"
    }
  ]
}
`, name)
}

func TestAccProfileResource_WithAllFields(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	profileName := fmt.Sprintf("tf-acc-allfields-%s", rName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with all fields
			{
				Config: testAccProfileResourceConfigAllFields(profileName, "Test profile description", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("name"),
						knownvalue.StringExact(profileName),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Test profile description"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("icon"),
						knownvalue.StringExact("🤖"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("system_prompt"),
						knownvalue.StringExact("You are a helpful assistant"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("consider_context_untrusted"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("agent_type"),
						knownvalue.StringExact("agent"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_profile.allfields",
				ImportState:       true,
				ImportStateVerify: true,
				// Labels order may differ on import; suggested_prompts not set so no issue
				ImportStateVerifyIgnore: []string{"labels"},
			},
			// Update: change description and consider_context_untrusted
			{
				Config: testAccProfileResourceConfigAllFields(profileName, "Updated description", false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Updated description"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("consider_context_untrusted"),
						knownvalue.Bool(false),
					),
					// Verify unchanged fields persist
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("icon"),
						knownvalue.StringExact("🤖"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("system_prompt"),
						knownvalue.StringExact("You are a helpful assistant"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.allfields",
						tfjsonpath.New("agent_type"),
						knownvalue.StringExact("agent"),
					),
				},
			},
		},
	})
}

func TestAccProfileResource_WithEmailConfig(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	profileName := fmt.Sprintf("tf-acc-email-%s", rName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with incoming_email_enabled = true and security_mode = "public"
			{
				Config: testAccProfileResourceConfigWithEmail(profileName, true, "public"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile.email_test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(profileName),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.email_test",
						tfjsonpath.New("incoming_email_enabled"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.email_test",
						tfjsonpath.New("incoming_email_security_mode"),
						knownvalue.StringExact("public"),
					),
				},
			},
			// Update: change security_mode to "private"
			{
				Config: testAccProfileResourceConfigWithEmail(profileName, true, "private"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_profile.email_test",
						tfjsonpath.New("incoming_email_enabled"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.email_test",
						tfjsonpath.New("incoming_email_security_mode"),
						knownvalue.StringExact("private"),
					),
				},
			},
		},
	})
}

func testAccProfileResourceConfigWithEmail(name string, emailEnabled bool, securityMode string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "email_test" {
  name                        = %[1]q
  incoming_email_enabled       = %[2]t
  incoming_email_security_mode = %[3]q
}
`, name, emailEnabled, securityMode)
}

func testAccProfileResourceConfigAllFields(name string, description string, considerContextUntrusted bool) string {
	return fmt.Sprintf(`
resource "archestra_profile" "allfields" {
  name                       = %[1]q
  description                = %[2]q
  icon                       = "🤖"
  system_prompt              = "You are a helpful assistant"
  consider_context_untrusted = %[3]t
  agent_type                 = "agent"

  labels = [
    {
      key   = "env"
      value = "test"
    }
  ]
}
`, name, description, considerContextUntrusted)
}
