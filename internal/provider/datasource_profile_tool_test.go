package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccProfileToolDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProfileToolDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_profile_tool.test",
						tfjsonpath.New("tool_name"),
						knownvalue.StringExact("read_file"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_profile_tool.test",
						tfjsonpath.New("tool_result_treatment"),
						knownvalue.StringExact("untrusted"),
					),
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

func testAccProfileToolDataSourceConfig() string {
	return `
# Create a profile for testing
resource "archestra_profile" "test" {
  name = "profile-tool-ds-test-profile"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "test" {
  name        = "profile-tool-ds-test-server"
  description = "MCP server for profile tool data source testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "profile-tool-ds-installation"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up the profile tool
data "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_name  = "read_file"

  depends_on = [archestra_mcp_server_installation.test]
}
`
}
