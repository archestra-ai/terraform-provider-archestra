package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTrustedDataPolicyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTrustedDataPolicyConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("mark_as_trusted"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("attribute_path"),
						knownvalue.StringExact("content"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("contains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("trusted"),
					),
				},
			},
		},
	})
}

func testAccTrustedDataPolicyConfig() string {
	return `
# Create a test team
resource "archestra_team" "test" {
  name        = "Policy Test Team Data"
  description = "Team for policy testing"
}

# Create a test profile
resource "archestra_profile" "test" {
  name = "policy-test-profile-data"
}

# Create an MCP server
resource "archestra_mcp_server" "test" {
  name        = "policy-test-server-data"
  description = "MCP server for policy testing"
  docs_url    = "https://github.com/example/policy-test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "policy-test-installation-data"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up a tool from the profile
data "archestra_profile_tool" "lookup" {
  profile_id = archestra_profile.test.id
  tool_name  = "read_file"
  depends_on = [archestra_mcp_server_installation.test]
}

# Create a trusted data policy
resource "archestra_trusted_data_policy" "test" {
  profile_tool_id = data.archestra_profile_tool.lookup.id
  attribute_path  = "content"
  operator        = "contains"
  value           = "trusted"
  action          = "mark_as_trusted"
  description     = "Mark specific file content as trusted"
}
`
}
