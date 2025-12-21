package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccArchestraPromptResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccArchestraPromptResourceConfig("test-prompt", "A test prompt"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "profile_id", "archestra_profile.test", "id"),
					resource.TestCheckResourceAttr("archestra_prompt.test", "name", "test-prompt"),
					resource.TestCheckResourceAttr("archestra_prompt.test", "system_prompt", "A system prompt"),
					resource.TestCheckResourceAttr("archestra_prompt.test", "prompt", "the main prompt"),
					resource.TestCheckResourceAttrSet("archestra_prompt.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccArchestraPromptResourceConfig("updated-prompt", "Updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_prompt.test", "name", "updated-prompt"),
					resource.TestCheckResourceAttr("archestra_prompt.test", "prompt", "Updated prompt"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccArchestraPromptResourceConfig(name, prompt string) string {
	return fmt.Sprintf(`
resource "archestra_prompt" "test" {
  name          = %[1]q
  prompt        = %[2]q
  system_prompt = "the system prompt"
}
`, name, prompt)
}
