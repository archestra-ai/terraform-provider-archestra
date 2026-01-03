package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccPromptResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccPromptResourceConfig("test-prompt", "system prompt", "user prompt"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_prompt.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-prompt"),
					),
					statecheck.ExpectKnownValue(
						"archestra_prompt.test",
						tfjsonpath.New("system_prompt"),
						knownvalue.StringExact("system prompt"),
					),
					statecheck.ExpectKnownValue(
						"archestra_prompt.test",
						tfjsonpath.New("user_prompt"),
						knownvalue.StringExact("user prompt"),
					),
					statecheck.ExpectKnownValue(
						"archestra_prompt.test",
						tfjsonpath.New("is_active"),
						knownvalue.Bool(true),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_prompt.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccPromptResourceConfig("test-prompt-updated", "system prompt updated", "user prompt updated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_prompt.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-prompt-updated"),
					),
					statecheck.ExpectKnownValue(
						"archestra_prompt.test",
						tfjsonpath.New("system_prompt"),
						knownvalue.StringExact("system prompt updated"),
					),
					statecheck.ExpectKnownValue(
						"archestra_prompt.test",
						tfjsonpath.New("user_prompt"),
						knownvalue.StringExact("user prompt updated"),
					),
				},
			},
		},
	})
}

func testAccPromptResourceConfig(name, system, user string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "Profile for Prompt Test"
}

resource "archestra_prompt" "test" {
  profile_id    = archestra_profile.test.id
  name          = %[1]q
  system_prompt = %[2]q
  user_prompt   = %[3]q
}
`, name, system, user)
}
