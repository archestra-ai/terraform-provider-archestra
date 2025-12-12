package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOptimizationRuleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccOptimizationRuleResourceConfig("openai", "gpt-4o-mini", 500),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "entity_id", "default-org"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "entity_type", "organization"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "llm_provider", "openai"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "target_model", "gpt-4o-mini"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "enabled", "true"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "conditions.0.max_length", "500"),
					resource.TestCheckResourceAttrSet("archestra_optimization_rule.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:            "archestra_optimization_rule.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"conditions"},
			},
			// Update and Read testing
			{
				Config: testAccOptimizationRuleResourceConfig("openai", "gpt-3.5-turbo", 1000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "target_model", "gpt-3.5-turbo"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "conditions.0.max_length", "1000"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccOptimizationRuleResourceWithHasTools(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with has_tools condition
			{
				Config: testAccOptimizationRuleResourceConfigWithHasTools("anthropic", "claude-3-haiku-20240307", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "llm_provider", "anthropic"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "target_model", "claude-3-haiku-20240307"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "conditions.0.has_tools", "false"),
					resource.TestCheckResourceAttrSet("archestra_optimization_rule.test", "id"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccOptimizationRuleResourceDisabled(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with enabled = false
			{
				Config: testAccOptimizationRuleResourceConfigDisabled("gemini", "gemini-1.5-flash", 200),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "llm_provider", "gemini"),
					resource.TestCheckResourceAttr("archestra_optimization_rule.test", "enabled", "false"),
					resource.TestCheckResourceAttrSet("archestra_optimization_rule.test", "id"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccOptimizationRuleResourceConfig(provider, targetModel string, maxLength int) string {
	return fmt.Sprintf(`
resource "archestra_optimization_rule" "test" {
  entity_id    = "default-org"
  entity_type  = "organization"
  llm_provider = %[1]q
  target_model = %[2]q
  enabled      = true
  conditions   = [
    {
      max_length = %[3]d
    }
  ]
}
`, provider, targetModel, maxLength)
}

func testAccOptimizationRuleResourceConfigWithHasTools(provider, targetModel string, hasTools bool) string {
	return fmt.Sprintf(`
resource "archestra_optimization_rule" "test" {
  entity_id    = "default-org"
  entity_type  = "organization"
  llm_provider = %[1]q
  target_model = %[2]q
  enabled      = true
  conditions   = [
    {
      has_tools = %[3]t
    }
  ]
}
`, provider, targetModel, hasTools)
}

func testAccOptimizationRuleResourceConfigDisabled(provider, targetModel string, maxLength int) string {
	return fmt.Sprintf(`
resource "archestra_optimization_rule" "test" {
  entity_id    = "default-org"
  entity_type  = "organization"
  llm_provider = %[1]q
  target_model = %[2]q
  enabled      = false
  conditions   = [
    {
      max_length = %[3]d
    }
  ]
}
`, provider, targetModel, maxLength)
}
