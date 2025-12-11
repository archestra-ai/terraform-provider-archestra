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
						tfjsonpath.New("description"),
						knownvalue.StringExact("Trust internal API responses"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("attribute_path"),
						knownvalue.StringExact("url"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("contains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("api.internal.example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("mark_as_trusted"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_trusted_data_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccTrustedDataPolicyResourceConfigUpdated(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Block untrusted external data"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("attribute_path"),
						knownvalue.StringExact("source"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("notContains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTrustedDataPolicyResource_SanitizeAction(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with sanitize_with_dual_llm action
			{
				Config: testAccTrustedDataPolicyResourceConfigSanitize(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy.sanitize",
						tfjsonpath.New("action"),
						knownvalue.StringExact("sanitize_with_dual_llm"),
					),
				},
			},
		},
	})
}

func testAccTrustedDataPolicyResourceConfig() string {
	return `
# Create an agent for testing
resource "archestra_agent" "test" {
  name = "trusted-data-policy-test-agent"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "test" {
  name        = "trusted-data-policy-test-server"
  description = "MCP server for trusted data policy testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "trusted-data-policy-installation"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up the agent tool
data "archestra_agent_tool" "test" {
  agent_id  = archestra_agent.test.id
  tool_name = "archestra__whoami"

  depends_on = [archestra_mcp_server_installation.test]
}

# Create a trusted data policy
resource "archestra_trusted_data_policy" "test" {
  agent_tool_id  = data.archestra_agent_tool.test.id
  description    = "Trust internal API responses"
  attribute_path = "url"
  operator       = "contains"
  value          = "api.internal.example.com"
  action         = "mark_as_trusted"
}
`
}

func testAccTrustedDataPolicyResourceConfigUpdated() string {
	return `
# Create an agent for testing
resource "archestra_agent" "test" {
  name = "trusted-data-policy-test-agent"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "test" {
  name        = "trusted-data-policy-test-server"
  description = "MCP server for trusted data policy testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "trusted-data-policy-installation"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up the agent tool
data "archestra_agent_tool" "test" {
  agent_id  = archestra_agent.test.id
  tool_name = "archestra__whoami"

  depends_on = [archestra_mcp_server_installation.test]
}

# Create a trusted data policy (updated)
resource "archestra_trusted_data_policy" "test" {
  agent_tool_id  = data.archestra_agent_tool.test.id
  description    = "Block untrusted external data"
  attribute_path = "source"
  operator       = "notContains"
  value          = "example.com"
  action         = "block_always"
}
`
}

func testAccTrustedDataPolicyResourceConfigSanitize() string {
	return `
# Create an agent for testing
resource "archestra_agent" "sanitize" {
  name = "trusted-data-policy-sanitize-agent"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "sanitize" {
  name        = "trusted-data-policy-sanitize-server"
  description = "MCP server for sanitize action testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "sanitize" {
  name          = "trusted-data-sanitize-installation"
  mcp_server_id = archestra_mcp_server.sanitize.id
}

# Look up the agent tool
data "archestra_agent_tool" "sanitize" {
  agent_id  = archestra_agent.sanitize.id
  tool_name = "archestra__whoami"

  depends_on = [archestra_mcp_server_installation.sanitize]
}

# Create a trusted data policy with sanitize action
resource "archestra_trusted_data_policy" "sanitize" {
  agent_tool_id  = data.archestra_agent_tool.sanitize.id
  description    = "Sanitize user input with dual LLM"
  attribute_path = "user_input"
  operator       = "regex"
  value          = ".*"
  action         = "sanitize_with_dual_llm"
}
`
}
