package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccDualLlmConfigResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccDualLlmConfigResourceConfig(
					"Main agent prompt for testing",
					"Quarantined agent prompt for testing",
					"Summary prompt for testing",
					true,
					5,
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("main_agent_prompt"),
						knownvalue.StringExact("Main agent prompt for testing"),
					),
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("quarantined_agent_prompt"),
						knownvalue.StringExact("Quarantined agent prompt for testing"),
					),
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("summary_prompt"),
						knownvalue.StringExact("Summary prompt for testing"),
					),
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("enabled"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("max_rounds"),
						knownvalue.Int64Exact(5),
					),
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_dual_llm_config.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccDualLlmConfigResourceConfig(
					"Updated main agent prompt",
					"Updated quarantined agent prompt",
					"Updated summary prompt",
					false,
					10,
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("main_agent_prompt"),
						knownvalue.StringExact("Updated main agent prompt"),
					),
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("enabled"),
						knownvalue.Bool(false),
					),
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.test",
						tfjsonpath.New("max_rounds"),
						knownvalue.Int64Exact(10),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccDualLlmConfigResource_Minimal(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with only required fields
			{
				Config: testAccDualLlmConfigResourceConfigMinimal(
					"Minimal main prompt",
					"Minimal quarantined prompt",
					"Minimal summary prompt",
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.minimal",
						tfjsonpath.New("main_agent_prompt"),
						knownvalue.StringExact("Minimal main prompt"),
					),
					statecheck.ExpectKnownValue(
						"archestra_dual_llm_config.minimal",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_dual_llm_config.minimal",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDualLlmConfigResourceConfig(mainPrompt, quarantinedPrompt, summaryPrompt string, enabled bool, maxRounds int) string {
	return fmt.Sprintf(`
resource "archestra_dual_llm_config" "test" {
  main_agent_prompt        = %[1]q
  quarantined_agent_prompt = %[2]q
  summary_prompt           = %[3]q
  enabled                  = %[4]t
  max_rounds               = %[5]d
}
`, mainPrompt, quarantinedPrompt, summaryPrompt, enabled, maxRounds)
}

func testAccDualLlmConfigResourceConfigMinimal(mainPrompt, quarantinedPrompt, summaryPrompt string) string {
	return fmt.Sprintf(`
resource "archestra_dual_llm_config" "minimal" {
  main_agent_prompt        = %[1]q
  quarantined_agent_prompt = %[2]q
  summary_prompt           = %[3]q
}
`, mainPrompt, quarantinedPrompt, summaryPrompt)
}
