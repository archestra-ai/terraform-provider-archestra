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
			// Create and Read testing
			{
				Config: testAccTrustedDataPolicyResourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("equal"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("trust"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_trusted_data_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccTrustedDataPolicyResourceConfig() string {
	return `
# Create an agent first
resource "archestra_agent" "test_agent" {
  name = "trusted-data-test-agent"
}

# Create an MCP server
resource "archestra_mcp_server" "test_server" {
  name        = "trusted-data-test-server"
  description = "MCP server for trusted data testing"
  docs_url    = "https://github.com/example/trusted-data-test-server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server to get tools
resource "archestra_mcp_server_installation" "test_installation" {
  name          = "trusted-data-test-installation"
  mcp_server_id = archestra_mcp_server.test_server.id
}

# Get the agent tool for the policy
data "archestra_agent_tool" "test_tool" {
  agent_id = archestra_agent.test_agent.id
  name     = "list_directory"
  depends_on = [archestra_mcp_server_installation.test_installation]
}

# Create a trusted data policy
resource "archestra_trusted_data_policy" "test" {
  agent_tool_id    = data.archestra_agent_tool.test_tool.id
  description      = "Trust files from safe directories"
  attribute_path   = "result.files.*.path"
  operator         = "equal"
  value            = "/home/user/documents"
  action           = "trust"
}
`
}
