package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccPromptDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptDataSourceConfig("ds-test-prompt"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_prompt.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("ds-test-prompt"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_prompt.test_by_name",
						tfjsonpath.New("name"),
						knownvalue.StringExact("ds-test-prompt"),
					),
				},
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
