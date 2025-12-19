package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTrustedDataPolicyResource(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTrustedDataPolicyResourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Trust internal API responses"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("attribute_path"),
						knownvalue.StringExact("url"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("contains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("api.internal.example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("mark_as_trusted"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_trusted_data_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccTrustedDataPolicyResourceConfigUpdated(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Block untrusted external data"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("attribute_path"),
						knownvalue.StringExact("source"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("notContains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTrustedDataPolicyResource_SanitizeAction(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with sanitize_with_dual_llm action
			{
				Config: testAccTrustedDataPolicyResourceConfigSanitize(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.sanitize",
						tfjsonpath.New("action"),
						knownvalue.StringExact("sanitize_with_dual_llm"),
					),
				},
			},
		},
	})
}

// testAccTrustedDataPolicyResourceConfig creates a config using only the built-in
// archestra__whoami tool which is immediately available after agent creation.
func testAccTrustedDataPolicyResourceConfig(rName string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name = "tdp-test-agent-%[1]s"
}

# archestra__whoami is a built-in tool assigned synchronously when the profile is created.
# No MCP server or installation needed - the tool is immediately available.
data "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test.id
  tool_name  = "archestra__whoami"
}

resource "archestra_trusted_data_policy" "test" {
  profile_tool_id = data.archestra_profile_tool.test.id
  description     = "Trust internal API responses"
  attribute_path = "url"
  operator       = "contains"
  value          = "api.internal.example.com"
  action         = "mark_as_trusted"
}
`, rName)
}

func testAccTrustedDataPolicyResourceConfigUpdated(rName string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name = "tdp-test-agent-%[1]s"
}

# archestra__whoami is a built-in tool assigned synchronously when the profile is created.
# No MCP server or installation needed - the tool is immediately available.
data "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test.id
  tool_name  = "archestra__whoami"
}

resource "archestra_trusted_data_policy" "test" {
  profile_tool_id = data.archestra_profile_tool.test.id
  description     = "Block untrusted external data"
  attribute_path = "source"
  operator       = "notContains"
  value          = "example.com"
  action         = "block_always"
}
`, rName)
}

func testAccTrustedDataPolicyResourceConfigSanitize(rName string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "sanitize" {
  name = "tdp-sanitize-agent-%[1]s"
}

# archestra__whoami is a built-in tool assigned synchronously when the profile is created.
# No MCP server or installation needed - the tool is immediately available.
data "archestra_profile_tool" "sanitize" {
  profile_id = archestra_agent.sanitize.id
  tool_name  = "archestra__whoami"
}

resource "archestra_trusted_data_policy" "sanitize" {
  profile_tool_id = data.archestra_profile_tool.sanitize.id
  description     = "Sanitize user input with dual LLM"
  attribute_path = "user_input"
  operator       = "regex"
  value          = ".*"
  action         = "sanitize_with_dual_llm"
}
`, rName)
}
