package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccArchestraPromptDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"archestra": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccPromptDataSourceConfig("test-prompt-ds"),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify data source resolved correctly
					resource.TestCheckResourceAttr("data.archestra_archestra_prompt.test", "name", "test-prompt-ds"),
					resource.TestCheckResourceAttr("data.archestra_archestra_prompt.test", "prompt", "Hello from prompt"),
					resource.TestCheckResourceAttr("data.archestra_archestra_prompt.test", "system_prompt", "System says hi"),
					resource.TestCheckResourceAttr("data.archestra_archestra_prompt.test", "is_active", "true"),

					// Computed fields must exist
					resource.TestCheckResourceAttrSet("data.archestra_archestra_prompt.test", "prompt_id"),
					resource.TestCheckResourceAttrSet("data.archestra_archestra_prompt.test", "profile_id"),
					resource.TestCheckResourceAttrSet("data.archestra_archestra_prompt.test", "version"),
				),
			},
		},
	})
}

func testAccPromptDataSourceConfig(promptName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "test-profile-for-prompt-ds"
}

resource "archestra_prompt" "test" {
  profile_id    = archestra_profile.test.id
  name          = "%[1]s"
  prompt        = "Hello from prompt"
  system_prompt = "System says hi"
  is_active     = true
}

data "archestra_archestra_prompt" "test" {
  name = archestra_prompt.test.name
}

data "archestra_archestra_prompt" "test_by_id" {
  prompt_id = archestra_prompt.test.prompt_id
}
`, promptName)
}
