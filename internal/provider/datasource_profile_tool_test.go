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
			// Create profile and look up the built-in tool
			{
				Config: testAccProfileToolDataSourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify the data source returns the expected values
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
					statecheck.ExpectKnownValue(
						"data.archestra_profile_tool.test",
						tfjsonpath.New("tool_result_treatment"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

// testAccProfileToolDataSourceConfig creates a minimal config to test the profile_tool
// data source using the built-in archestra__whoami tool which is immediately
// available after profile creation (no MCP server needed).
func testAccProfileToolDataSourceConfig(rName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "profile-tool-ds-test-%[1]s"
}

# archestra__whoami is a built-in tool assigned synchronously when the profile is created.
# No MCP server or installation needed - the tool is immediately available.
data "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_name  = "archestra__whoami"
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
				ExpectError: regexp.MustCompile(`not found`),
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
