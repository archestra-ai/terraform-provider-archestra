package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccToolInvocationPolicyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolInvocationPolicyConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("argument_name"),
						knownvalue.StringExact("path"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("contains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("/etc"),
					),
				},
			},
		},
	})
}

func testAccToolInvocationPolicyConfig() string {
	return `
# Create a test team
resource "archestra_team" "test" {
  name        = "Policy Test Team"
  description = "Team for policy testing"
}

# Create a test profile
resource "archestra_profile" "test" {
  name = "policy-test-profile"
}

# Create an MCP server
resource "archestra_mcp_server" "test" {
  name        = "policy-test-server"
  description = "MCP server for policy testing"
  docs_url    = "https://github.com/example/policy-test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "policy-test-installation"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up a tool from the profile
data "archestra_profile_tool" "lookup" {
  profile_id = archestra_profile.test.id
  tool_name  = "read_file"
  depends_on = [archestra_mcp_server_installation.test]
}

# Create a tool invocation policy
resource "archestra_tool_invocation_policy" "test" {
  profile_tool_id = data.archestra_profile_tool.lookup.id
  argument_name   = "path"
  operator        = "contains"
  value           = "/etc"
  action          = "block_always"
  description     = "Block access to system files"
}
`
}
