package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAgentToolDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create agent, MCP server, and installation, then look up the tool
			{
				Config: testAccAgentToolDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify the data source returns the expected values
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tool.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tool.test",
						tfjsonpath.New("tool_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tool.test",
						tfjsonpath.New("tool_result_treatment"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func testAccAgentToolDataSourceConfig() string {
	return `
# Create an agent for testing
resource "archestra_agent" "test" {
  name = "agent-tool-datasource-test"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "test" {
  name        = "agent-tool-test-server"
  description = "MCP server for agent tool data source testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server (this makes tools available)
resource "archestra_mcp_server_installation" "test" {
  name          = "agent-tool-installation"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up a tool from the installed MCP server
# Note: The filesystem MCP server provides tools like "read_file", "write_file", etc.
# We use the agent's built-in Archestra tools which are always available
data "archestra_agent_tool" "test" {
  agent_id  = archestra_agent.test.id
  tool_name = "archestra__whoami"

  depends_on = [archestra_mcp_server_installation.test]
}
`
}

func TestAccAgentToolDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccAgentToolDataSourceConfigNotFound(),
				ExpectError: regexp.MustCompile(`not found`),
			},
		},
	})
}

func testAccAgentToolDataSourceConfigNotFound() string {
	return `
resource "archestra_agent" "test" {
  name = "agent-tool-notfound-test"
}

data "archestra_agent_tool" "test" {
  agent_id  = archestra_agent.test.id
  tool_name = "nonexistent_tool_that_does_not_exist"
}
`
}
