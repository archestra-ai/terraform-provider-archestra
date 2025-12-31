package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccArchestraPromptVersionsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"archestra": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccPromptVersionsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(
						"data.archestra_archestra_prompt_versions.test",
						"versions.#",
					),
				),
			},
		},
	})
}

func testAccPromptVersionsDataSourceConfig() string {
	return `
resource "archestra_profile" "test" {
  name = "versions-profile"
}

resource "archestra_prompt" "test" {
  profile_id = archestra_profile.test.id
  name       = "versioned-prompt"
  prompt     = "v1"
}

# Update creates a new version
resource "archestra_prompt" "update" {
  profile_id = archestra_profile.test.id
  name       = "versioned-prompt"
  prompt     = "v2"

  depends_on = [archestra_prompt.test]
}

data "archestra_archestra_prompt_versions" "test" {
  prompt_id = archestra_prompt.update.prompt_id
}
`
}


