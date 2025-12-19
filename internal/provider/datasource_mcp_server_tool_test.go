package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccMCPServerToolDataSource(t *testing.T) {
	// This test validates that after MCP server installation, tools become available
	// and can be looked up using the archestra_mcp_server_tool data source.
	// The MCP server installation resource waits for tools to be ready before completing.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMCPServerToolDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_server_tool.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_server_tool.test",
						tfjsonpath.New("name"),
						knownvalue.StringRegexp(regexp.MustCompile(`__list_directory$`)),
					),
				},
			},
		},
	})
}

func testAccMCPServerToolDataSourceConfig() string {
	return `
# Create an MCP server in the registry
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "test-mcp-server-for-tool-datasource"
  description = "MCP server for tool data source test"
  docs_url    = "https://github.com/modelcontextprotocol/servers"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server (this now waits for tools to be available)
resource "archestra_mcp_server_installation" "test" {
  name          = "test-tool-datasource-installation"
  mcp_server_id = archestra_mcp_registry_catalog_item.test.id
}

# Look up a tool from the installed MCP server
# Tool names are prefixed with the server name, e.g., "servername__toolname"
data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server_installation.test.id
  name          = "${archestra_mcp_registry_catalog_item.test.name}__list_directory"
}
`
}
