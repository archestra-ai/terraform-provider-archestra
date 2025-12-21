package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccArchestraPromptDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccArchestraPromptDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.archestra_prompt.test", "name", "test-prompt"),
					resource.TestCheckResourceAttrSet("data.archestra_prompt.test", "id"),
				),
			},
		},
	})
}

const testAccArchestraPromptDataSourceConfig = `
data "archestra_prompt" "test" {
  name = "test-prompt"
}
`
