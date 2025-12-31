package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPromptDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptDataSourceConfig("ds-test-prompt"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.archestra_prompt.test", "name", "ds-test-prompt"),
					resource.TestCheckResourceAttrSet("data.archestra_prompt.test", "id"),
					resource.TestCheckResourceAttrSet("data.archestra_prompt.test", "profile_id"),
					resource.TestCheckResourceAttr("data.archestra_prompt.test_by_name", "name", "ds-test-prompt"),
				),
			},
		},
	})
}

func testAccPromptDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "Profile for Prompt DS Test"
}

resource "archestra_prompt" "test" {
  profile_id    = archestra_profile.test.id
  name          = %[1]q
  system_prompt = "system"
  user_prompt   = "user"
}

data "archestra_prompt" "test" {
  id = archestra_prompt.test.id
}

data "archestra_prompt" "test_by_name" {
  name = archestra_prompt.test.name
}
`, name)
}

func TestAccPromptVersionsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptVersionsDataSourceConfig("versions-test-prompt"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_prompt_versions.test", "prompt_id"),
					resource.TestCheckResourceAttr("data.archestra_prompt_versions.test", "versions.#", "1"),
				),
			},
		},
	})
}

func testAccPromptVersionsDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "Profile for Prompt Versions DS Test"
}

resource "archestra_prompt" "test" {
  profile_id    = archestra_profile.test.id
  name          = %[1]q
  system_prompt = "v1"
}

data "archestra_prompt_versions" "test" {
  prompt_id = archestra_prompt.test.id
}
`, name)
}
