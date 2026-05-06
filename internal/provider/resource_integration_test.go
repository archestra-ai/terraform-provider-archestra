package provider

import (
	"regexp"
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

// TestAccIntegration_CrossScope_TeamAgentPersonalInstall pins the backend
// rule in services/agent-tool-assignment.ts isMcpServerAssignableToTarget:
// a team-scoped agent binding a tool from a personal-owned install
// (ownerId set, teamId null) is allowed only when the install's owner is
// a member of the agent's teams. The Terraform-created team has no
// members, so the API key user (who owns the install) isn't in it, and
// the assignment is rejected with the canonical message.
func TestAccIntegration_CrossScope_TeamAgentPersonalInstall(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCrossScopeConfig(),
				ExpectError: regexp.MustCompile(`credential owner must be a member of a team`),
			},
		},
	})
}

func testAccCrossScopeConfig() string {
	return `
resource "archestra_team" "cross" {
  name        = "cross-scope-team"
  description = "Cross-scope binding test"
}

resource "archestra_agent" "team_scoped" {
  name          = "tf-acc-cross-scope-team-agent"
  system_prompt = "Team-scoped agent."
  scope         = "team"
  teams         = [archestra_team.cross.id]
}

resource "archestra_mcp_registry_catalog_item" "cross" {
  name        = "tf-acc-cross-scope-server"
  description = "Cross-scope binding test catalog"
  docs_url    = "https://example.com/cross"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Personal-scoped install: deliberately no team_id.
resource "archestra_mcp_server_installation" "personal" {
  name       = "tf-acc-cross-scope-personal-install"
  catalog_id = archestra_mcp_registry_catalog_item.cross.id
}

# The actual cross-scope binding: a team-scoped agent picking up a tool
# from a personal-scoped install. Pulls the first tool the install
# advertises (filesystem servers expose several; any one works).
resource "archestra_agent_tool" "cross" {
  agent_id      = archestra_agent.team_scoped.id
  tool_id       = archestra_mcp_server_installation.personal.tools[0].id
  mcp_server_id = archestra_mcp_server_installation.personal.id
}
`
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
