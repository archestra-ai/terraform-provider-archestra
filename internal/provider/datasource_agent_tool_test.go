package provider

import (
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
			{
				Config: testAccAgentToolDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tool.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("list_directory"),
					),
				},
			},
		},
	})
}

func testAccAgentToolDataSourceConfig() string {
	return `
# Create an agent
resource "archestra_agent" "test" {
  name = "agent-tool-datasource-test"
}

# Create an MCP server
resource "archestra_mcp_server" "test" {
  name        = "agent-tool-datasource-server"
  description = "MCP server for agent tool data source testing"
  docs_url    = "https://github.com/example/agent-tool-test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server to get tools
resource "archestra_mcp_server_installation" "test" {
  name          = "agent-tool-datasource-installation"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up a specific agent tool
data "archestra_agent_tool" "test" {
  agent_id = archestra_agent.test.id
  name     = "list_directory"
  depends_on = [archestra_mcp_server_installation.test]
}
`
}
