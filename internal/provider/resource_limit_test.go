package provider

import (
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
				Config: testAccLimitResourceConfig("test-org", "organization", "token_cost", "100000", `["gpt-4o"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.test", "entity_id", "test-org"),
					resource.TestCheckResourceAttr("archestra_limit.test", "entity_type", "organization"),
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_type", "token_cost"),
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_value", "100000"),
					resource.TestCheckResourceAttr("archestra_limit.test", "model.0", "gpt-4o"),
					resource.TestCheckResourceAttrSet("archestra_limit.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccLimitResourceConfig("test-org", "organization", "token_cost", "200000", `["gpt-4o", "claude-3-opus-20240229"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.test", "limit_value", "200000"),
					resource.TestCheckResourceAttr("archestra_limit.test", "model.#", "2"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccLimitResourceConfig(entityID, entityType, limitType, limitValue, models string) string {
	return `
resource "archestra_limit" "test" {
  entity_id   = "` + entityID + `"
  entity_type = "` + entityType + `"
  limit_type  = "` + limitType + `"
  limit_value = ` + limitValue + `
  model       = ` + models + `
}
`
}
