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
			// Create team, agent, MCP server, and installation
			{
				Config: testAccIntegrationConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Team checks
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("Integration Test Team"),
					),
					// Agent checks
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("integration-test-profile"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("environment"),
					),
					statecheck.ExpectKnownValue(
						"archestra_profile.test",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("value"),
						knownvalue.StringExact("test"),
					),
					// MCP Server checks
					statecheck.ExpectKnownValue(
						"archestra_mcp_server.test",
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
					// Verify profile tool lookup works
					statecheck.ExpectKnownValue(
						"data.archestra_profile_tool.lookup",
						tfjsonpath.New("tool_name"),
						knownvalue.StringExact("read_file"),
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

# Create a test profile with labels
resource "archestra_profile" "test" {
  name = "integration-test-profile"

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
resource "archestra_mcp_server" "test" {
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
  mcp_server_id = archestra_mcp_server.test.id
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

# Create a test profile with labels
resource "archestra_profile" "test" {
  name = "integration-test-profile"

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
resource "archestra_mcp_server" "test" {
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
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up a tool from the profile (filesystem server should provide tools)
# Note: we use depends_on to ensure installation is complete
data "archestra_profile_tool" "lookup" {
  profile_id = archestra_profile.test.id
  tool_name  = "read_file"
  depends_on = [archestra_mcp_server_installation.test]
}
`
}
