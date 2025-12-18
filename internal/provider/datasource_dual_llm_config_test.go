package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccDualLlmConfigDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create resource and read it via data source
			{
				Config: testAccDualLlmConfigDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Check that the data source returns the same data as the resource
					statecheck.ExpectKnownValue(
						"data.archestra_dual_llm_config.test",
						tfjsonpath.New("main_agent_prompt"),
						knownvalue.StringExact("Test main agent prompt"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_dual_llm_config.test",
						tfjsonpath.New("quarantined_agent_prompt"),
						knownvalue.StringExact("Test quarantined agent prompt"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_dual_llm_config.test",
						tfjsonpath.New("summary_prompt"),
						knownvalue.StringExact("Test summary prompt"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_dual_llm_config.test",
						tfjsonpath.New("enabled"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_dual_llm_config.test",
						tfjsonpath.New("max_rounds"),
						knownvalue.Int64Exact(3),
					),
					// Verify that the data source ID is a valid UUID
					statecheck.ExpectKnownValue(
						"data.archestra_dual_llm_config.test",
						tfjsonpath.New("id"),
						knownvalue.StringRegexp(regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)),
					),
				},
			},
		},
	})
}

func testAccDualLlmConfigDataSourceConfig() string {
	return `
# First create a dual LLM config
resource "archestra_dual_llm_config" "example" {
  main_agent_prompt        = "Test main agent prompt"
  quarantined_agent_prompt = "Test quarantined agent prompt"
  summary_prompt           = "Test summary prompt"
  enabled                  = true
  max_rounds               = 3
}

# Then read it via data source
data "archestra_dual_llm_config" "test" {
  id = archestra_dual_llm_config.example.id
}
`
}
