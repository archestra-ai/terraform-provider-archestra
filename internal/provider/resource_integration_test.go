package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccIntegration_FullWorkflow(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create team, MCP gateway, MCP server, and installation
			{
				Config: testAccIntegrationConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Team checks
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("Integration Test Team"),
					),
					// MCP gateway checks
					statecheck.ExpectKnownValue(
						"archestra_mcp_gateway.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("integration-test-mcp-gateway"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_gateway.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("environment"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_gateway.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("value"),
						knownvalue.StringExact("test"),
					),
					// MCP Server checks
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("integration-test-server"),
					),
					// MCP Server Installation checks
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("integration-test-installation"),
					),
					// display_name is the actual name from the API (may have suffix)
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("display_name"),
						knownvalue.NotNull(),
					),
				},
			},
			// Test data source integration
			{
				Config: testAccIntegrationConfigWithDataSources(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify data source returns same data as resource
					statecheck.ExpectKnownValue(
						"data.archestra_team.lookup",
						tfjsonpath.New("name"),
						knownvalue.StringExact("Integration Test Team"),
					),
				},
			},
		},
	})
}

func testAccIntegrationConfig() string {
	return `
# Create a test team
resource "archestra_team" "test" {
  name        = "Integration Test Team"
  description = "Team for integration testing"
}

# Create a test MCP gateway with labels
resource "archestra_mcp_gateway" "test" {
  name = "integration-test-mcp-gateway"

  labels = [
    {
      key   = "environment"
      value = "test"
    },
    {
      key   = "purpose"
      value = "integration-testing"
    }
  ]
}

# Create an MCP server
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "integration-test-server"
  description = "MCP server for integration testing"
  docs_url    = "https://github.com/example/integration-test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "integration-test-installation"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}
`
}

func testAccIntegrationConfigWithDataSources() string {
	return `
# Create a test team
resource "archestra_team" "test" {
  name        = "Integration Test Team"
  description = "Team for integration testing"
}

# Look up the team via data source
data "archestra_team" "lookup" {
  id = archestra_team.test.id
}

# Create a test MCP gateway with labels
resource "archestra_mcp_gateway" "test" {
  name = "integration-test-mcp-gateway"

  labels = [
    {
      key   = "environment"  
      value = "test"
    },
    {
      key   = "purpose"
      value = "integration-testing"
    }
  ]
}

# Create an MCP server
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "integration-test-server"
  description = "MCP server for integration testing"
  docs_url    = "https://github.com/example/integration-test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "integration-test-installation"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}
`
}
