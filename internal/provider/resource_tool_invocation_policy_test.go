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
			// Create and Read testing
			{
				Config: testAccToolInvocationPolicyResourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("equal"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_tool_invocation_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccToolInvocationPolicyResourceConfig() string {
	return `
# Create an agent first
resource "archestra_agent" "test_agent" {
  name = "policy-test-agent"
}

# Create an MCP server
resource "archestra_mcp_server" "test_server" {
  name        = "policy-test-server"
  description = "MCP server for policy testing"
  docs_url    = "https://github.com/example/policy-test-server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server to get tools
resource "archestra_mcp_server_installation" "test_installation" {
  name          = "policy-test-installation"
  mcp_server_id = archestra_mcp_server.test_server.id
}

# Get the agent tool for the policy
data "archestra_agent_tool" "test_tool" {
  agent_id = archestra_agent.test_agent.id
  name     = "list_directory"
  depends_on = [archestra_mcp_server_installation.test_installation]
}

# Create a tool invocation policy
resource "archestra_tool_invocation_policy" "test" {
  agent_tool_id  = data.archestra_agent_tool.test_tool.id
  argument_name  = "path"
  operator       = "equal"
  value          = "/etc"
  action         = "block_always"
  reason         = "Sensitive directory access blocked"
}
`
}
