package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccProfileToolResource(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProfileToolResourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("archestra_profile_tool.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_profile_tool.test", "profile_id"),
					resource.TestCheckResourceAttrSet("archestra_profile_tool.test", "tool_id"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "use_dynamic_team_credential", "false"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "allow_usage_when_untrusted_data_is_present", "false"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "tool_result_treatment", "trusted"),
				),
			},
			{
				ResourceName:      "archestra_profile_tool.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProfileToolImportStateIdFunc("archestra_profile_tool.test"),
			},
			{
				Config: testAccProfileToolResourceConfigUpdated(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "allow_usage_when_untrusted_data_is_present", "true"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "tool_result_treatment", "untrusted"),
				),
			},
		},
	})
}

func TestAccProfileToolResourceSanitize(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProfileToolResourceConfigSanitize(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "tool_result_treatment", "sanitize_with_dual_llm"),
				),
			},
		},
	})
}

func TestAccProfileToolResourceInvalidTreatment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProfileToolResourceConfigInvalidTreatment(),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func testAccProfileToolImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("not found: %s", resourceName)
		}
		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["profile_id"], rs.Primary.Attributes["tool_id"]), nil
	}
}

func testAccProfileToolResourceConfig(rName string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_server" "test" {
  name        = "profile-tool-test-server-%[1]s"
  description = "Test MCP server for profile tool testing"
  docs_url    = "https://github.com/example/test-server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "profile-tool-test-install-%[1]s"
  mcp_server_id = archestra_mcp_server.test.id
}

resource "archestra_agent" "test" {
  name = "profile-tool-test-agent-%[1]s"
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server.test.id
  name          = "read_file"
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test.id
  tool_id    = data.archestra_mcp_server_tool.test.id

  use_dynamic_team_credential              = false
  allow_usage_when_untrusted_data_is_present = false
  tool_result_treatment                    = "trusted"
}
`, rName)
}

func testAccProfileToolResourceConfigUpdated(rName string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_server" "test" {
  name        = "profile-tool-test-server-%[1]s"
  description = "Test MCP server for profile tool testing"
  docs_url    = "https://github.com/example/test-server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "profile-tool-test-install-%[1]s"
  mcp_server_id = archestra_mcp_server.test.id
}

resource "archestra_agent" "test" {
  name = "profile-tool-test-agent-%[1]s"
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server.test.id
  name          = "read_file"
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test.id
  tool_id    = data.archestra_mcp_server_tool.test.id

  use_dynamic_team_credential              = false
  allow_usage_when_untrusted_data_is_present = true
  tool_result_treatment                    = "untrusted"
}
`, rName)
}

func testAccProfileToolResourceConfigSanitize(rName string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_server" "test" {
  name        = "profile-tool-sanitize-server-%[1]s"
  description = "Test MCP server for profile tool sanitize testing"
  docs_url    = "https://github.com/example/test-server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "profile-tool-sanitize-install-%[1]s"
  mcp_server_id = archestra_mcp_server.test.id
}

resource "archestra_agent" "test" {
  name = "profile-tool-sanitize-agent-%[1]s"
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server.test.id
  name          = "read_file"
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test.id
  tool_id    = data.archestra_mcp_server_tool.test.id

  tool_result_treatment = "sanitize_with_dual_llm"
}
`, rName)
}

func testAccProfileToolResourceConfigInvalidTreatment() string {
	return `
resource "archestra_agent" "test" {
  name = "profile-tool-invalid-test"
}

resource "archestra_profile_tool" "test" {
  profile_id            = archestra_agent.test.id
  tool_id               = "00000000-0000-0000-0000-000000000000"
  tool_result_treatment = "invalid_treatment_value"
}
`
}
