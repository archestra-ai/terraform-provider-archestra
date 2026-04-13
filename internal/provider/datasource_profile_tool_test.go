package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccProfileToolDataSource(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProfileToolDataSourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_profile_tool.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_profile_tool.test",
						tfjsonpath.New("tool_id"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func testAccProfileToolDataSourceConfig(rName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "profile-tool-ds-test-%[1]s"
}

data "archestra_tool" "whoami" {
  name = "archestra__whoami"
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_id    = data.archestra_tool.whoami.id
}

data "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_name  = "archestra__whoami"
  depends_on = [archestra_profile_tool.test]
}
`, rName)
}

func TestAccProfileToolDataSource_NotFound(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProfileToolDataSourceConfigNotFound(rName),
				ExpectError: regexp.MustCompile(`not found|no tools assigned`),
			},
		},
	})
}

func testAccProfileToolDataSourceConfigNotFound(rName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "profile-tool-notfound-test-%[1]s"
}

data "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_name  = "nonexistent_tool_that_does_not_exist"
}
`, rName)
}
