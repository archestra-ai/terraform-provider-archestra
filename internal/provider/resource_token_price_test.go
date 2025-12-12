package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTokenPriceResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTokenPriceResourceConfig("openai", "gpt-4o", "2.50", "10.00"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_token_price.test", "llm_provider", "openai"),
					resource.TestCheckResourceAttr("archestra_token_price.test", "model", "gpt-4o"),
					resource.TestCheckResourceAttr("archestra_token_price.test", "price_per_million_input", "2.50"),
					resource.TestCheckResourceAttr("archestra_token_price.test", "price_per_million_output", "10.00"),
					resource.TestCheckResourceAttrSet("archestra_token_price.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccTokenPriceResourceConfig("openai", "gpt-4o", "3.00", "12.00"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_token_price.test", "price_per_million_input", "3.00"),
					resource.TestCheckResourceAttr("archestra_token_price.test", "price_per_million_output", "12.00"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccTokenPriceResourceConfig(provider, model, inputPrice, outputPrice string) string {
	return `
resource "archestra_token_price" "test" {
  llm_provider             = "` + provider + `"
  model                    = "` + model + `"
  price_per_million_input  = "` + inputPrice + `"
  price_per_million_output = "` + outputPrice + `"
}
`
}
