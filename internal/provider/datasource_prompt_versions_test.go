package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccPromptVersionsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptVersionsDataSourceConfig("versions-test-prompt"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_prompt_versions.test",
						tfjsonpath.New("versions"),
						knownvalue.ListSizeExact(1),
					),
				},
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
