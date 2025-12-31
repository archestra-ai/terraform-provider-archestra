package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPromptResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"archestra": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			// Create & Read
			{
				Config: testAccPromptResourceConfig("test-prompt"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"archestra_prompt.test",
						"profile_id",
						"archestra_profile.test",
						"id",
					),
					resource.TestCheckResourceAttr("archestra_prompt.test", "name", "test-prompt"),
					resource.TestCheckResourceAttr("archestra_prompt.test", "prompt", "Hello from user"),
					resource.TestCheckResourceAttr("archestra_prompt.test", "system_prompt", "You are helpful"),
					resource.TestCheckResourceAttrSet("archestra_prompt.test", "prompt_id"),
					resource.TestCheckResourceAttrSet("archestra_prompt.test", "version"),
				),
			},
			// Update (new version)
			{
				Config: testAccPromptResourceConfigUpdate("test-prompt"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_prompt.test", "name", "test-prompt"),
					resource.TestCheckResourceAttr("archestra_prompt.test", "prompt", "Updated user prompt"),
					resource.TestCheckResourceAttr("archestra_prompt.test", "system_prompt", "Updated system prompt"),
					resource.TestCheckResourceAttrSet("archestra_prompt.test", "prompt_id"),
					resource.TestCheckResourceAttrSet("archestra_prompt.test", "version"),
				),
			},
			// Import
			{
				ResourceName:      "archestra_prompt.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPromptResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "test-profile"
}

resource "archestra_prompt" "test" {
  profile_id    = archestra_profile.test.id
  name          = "%[1]s"
  prompt        = "Hello from user"
  system_prompt = "You are helpful"
}
`, name)
}

func testAccPromptResourceConfigUpdate(name string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "test-profile"
}

resource "archestra_prompt" "test" {
  profile_id    = archestra_profile.test.id
  name          = "%[1]s"
  prompt        = "Updated user prompt"
  system_prompt = "Updated system prompt"
}
`, name)
}
