package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccArchestraPromptVersionsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccArchestraPromptVersionsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_prompt_versions.test", "versions.#"),
				),
			},
		},
	})
}

const testAccArchestraPromptVersionsDataSourceConfig = `
data "archestra_prompt_versions" "test" {
  prompt_id = "example-id"
}
`
