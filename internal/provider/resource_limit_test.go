package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLimitResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccLimitResourceConfigTokenCost("test-org", "organization", "100000", `["gpt-4o"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.test", "entity_id", "test-org"),
					resource.TestCheckResourceAttr("archestra_limit.test", "entity_type", "organization"),
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_type", "token_cost"),
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_value", "100000"),
					resource.TestCheckResourceAttr("archestra_limit.test", "model.0", "gpt-4o"),
					resource.TestCheckResourceAttrSet("archestra_limit.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "archestra_limit.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccLimitResourceConfigTokenCost("test-org", "organization", "200000", `["gpt-4o", "claude-3-opus-20240229"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_value", "200000"),
					resource.TestCheckResourceAttr("archestra_limit.test", "model.#", "2"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccLimitResourceMCPServerCalls(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccLimitResourceConfigMCPServerCalls("test-org", "organization", "1000", "my-mcp-server"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.test", "entity_id", "test-org"),
					resource.TestCheckResourceAttr("archestra_limit.test", "entity_type", "organization"),
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_type", "mcp_server_calls"),
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_value", "1000"),
					resource.TestCheckResourceAttr("archestra_limit.test", "mcp_server_name", "my-mcp-server"),
					resource.TestCheckResourceAttrSet("archestra_limit.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "archestra_limit.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccLimitResourceConfigMCPServerCalls("test-org", "organization", "2000", "my-mcp-server"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_value", "2000"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccLimitResourceToolCalls(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccLimitResourceConfigToolCalls("test-org", "organization", "500", "my-mcp-server", "my-tool"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.test", "entity_id", "test-org"),
					resource.TestCheckResourceAttr("archestra_limit.test", "entity_type", "organization"),
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_type", "tool_calls"),
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_value", "500"),
					resource.TestCheckResourceAttr("archestra_limit.test", "mcp_server_name", "my-mcp-server"),
					resource.TestCheckResourceAttr("archestra_limit.test", "tool_name", "my-tool"),
					resource.TestCheckResourceAttrSet("archestra_limit.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "archestra_limit.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccLimitResourceConfigToolCalls("test-org", "organization", "750", "my-mcp-server", "my-tool"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_value", "750"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccLimitResourceConfigTokenCost(entityID, entityType, limitValue, models string) string {
	return fmt.Sprintf(`
resource "archestra_limit" "test" {
  entity_id   = %[1]q
  entity_type = %[2]q
  limit_type  = "token_cost"
  limit_value = %[3]s
  model       = %[4]s
}
`, entityID, entityType, limitValue, models)
}

func testAccLimitResourceConfigMCPServerCalls(entityID, entityType, limitValue, mcpServerName string) string {
	return fmt.Sprintf(`
resource "archestra_limit" "test" {
  entity_id       = %[1]q
  entity_type     = %[2]q
  limit_type      = "mcp_server_calls"
  limit_value     = %[3]s
  mcp_server_name = %[4]q
}
`, entityID, entityType, limitValue, mcpServerName)
}

func testAccLimitResourceConfigToolCalls(entityID, entityType, limitValue, mcpServerName, toolName string) string {
	return fmt.Sprintf(`
resource "archestra_limit" "test" {
  entity_id       = %[1]q
  entity_type     = %[2]q
  limit_type      = "tool_calls"
  limit_value     = %[3]s
  mcp_server_name = %[4]q
  tool_name       = %[5]q
}
`, entityID, entityType, limitValue, mcpServerName, toolName)
}
