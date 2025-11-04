package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccMCPServerToolDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMCPServerToolDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_server_tool.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("list_directory"),
					),
				},
			},
		},
	})
}

func testAccMCPServerToolDataSourceConfig() string {
	return `
# Create an MCP server
resource "archestra_mcp_server" "test" {
  name        = "mcp-server-tool-datasource-server"
  description = "MCP server for tool data source testing"
  docs_url    = "https://github.com/example/mcp-server-tool-test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Look up a specific MCP server tool
data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server.test.id
  name          = "list_directory"
}
`
}
