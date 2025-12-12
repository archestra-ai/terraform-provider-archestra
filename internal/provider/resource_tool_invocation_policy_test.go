package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccToolInvocationPolicyResource(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccToolInvocationPolicyResourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("argument_name"),
						knownvalue.StringExact("path"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("contains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("/etc/"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("reason"),
						knownvalue.StringExact("Block access to system configuration files"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_tool_invocation_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccToolInvocationPolicyResourceConfigUpdated(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("argument_name"),
						knownvalue.StringExact("path"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("startsWith"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("/var/log/"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("allow_when_context_is_untrusted"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("reason"),
						knownvalue.StringExact("Allow log file access in untrusted contexts"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccToolInvocationPolicyResource_WithoutReason(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create without optional reason field
			{
				Config: testAccToolInvocationPolicyResourceConfigNoReason(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.noreason",
						tfjsonpath.New("argument_name"),
						knownvalue.StringExact("command"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.noreason",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
				},
			},
		},
	})
}

func testAccToolInvocationPolicyResourceConfigNoReason(rName string) string {
	return fmt.Sprintf(`
# Create an agent for testing
resource "archestra_agent" "noreason" {
  name = "tip-noreason-agent-%[1]s"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "noreason" {
  name        = "tip-noreason-server-%[1]s"
  description = "MCP server for testing without reason"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "noreason" {
  name          = "tip-noreason-install-%[1]s"
  mcp_server_id = archestra_mcp_server.noreason.id
}

# Look up the agent tool
data "archestra_agent_tool" "noreason" {
  agent_id  = archestra_agent.noreason.id
  tool_name = "archestra__whoami"

  depends_on = [archestra_mcp_server_installation.noreason]
}

# Create a tool invocation policy without reason
resource "archestra_tool_invocation_policy" "noreason" {
  agent_tool_id = data.archestra_agent_tool.noreason.id
  argument_name = "command"
  operator      = "equal"
  value         = "rm -rf"
  action        = "block_always"
}
`, rName)
}

func TestAccToolInvocationPolicyResource_RegexOperator(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with regex operator
			{
				Config: testAccToolInvocationPolicyResourceConfigRegex(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.regex",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("regex"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.regex",
						tfjsonpath.New("value"),
						knownvalue.StringExact("^/home/[a-z]+/.ssh/.*"),
					),
				},
			},
		},
	})
}

func testAccToolInvocationPolicyResourceConfig(rName string) string {
	return fmt.Sprintf(`
# Create an agent for testing
resource "archestra_agent" "test" {
  name = "tip-test-agent-%[1]s"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "test" {
  name        = "tip-test-server-%[1]s"
  description = "MCP server for tool invocation policy testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "tip-test-install-%[1]s"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up the agent tool
data "archestra_agent_tool" "test" {
  agent_id  = archestra_agent.test.id
  tool_name = "archestra__whoami"

  depends_on = [archestra_mcp_server_installation.test]
}

# Create a tool invocation policy
resource "archestra_tool_invocation_policy" "test" {
  agent_tool_id = data.archestra_agent_tool.test.id
  argument_name = "path"
  operator      = "contains"
  value         = "/etc/"
  action        = "block_always"
  reason        = "Block access to system configuration files"
}
`, rName)
}

func testAccToolInvocationPolicyResourceConfigUpdated(rName string) string {
	return fmt.Sprintf(`
# Create an agent for testing
resource "archestra_agent" "test" {
  name = "tip-test-agent-%[1]s"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "test" {
  name        = "tip-test-server-%[1]s"
  description = "MCP server for tool invocation policy testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "test" {
  name          = "tip-test-install-%[1]s"
  mcp_server_id = archestra_mcp_server.test.id
}

# Look up the agent tool
data "archestra_agent_tool" "test" {
  agent_id  = archestra_agent.test.id
  tool_name = "archestra__whoami"

  depends_on = [archestra_mcp_server_installation.test]
}

# Create a tool invocation policy (updated)
resource "archestra_tool_invocation_policy" "test" {
  agent_tool_id = data.archestra_agent_tool.test.id
  argument_name = "path"
  operator      = "startsWith"
  value         = "/var/log/"
  action        = "allow_when_context_is_untrusted"
  reason        = "Allow log file access in untrusted contexts"
}
`, rName)
}

func testAccToolInvocationPolicyResourceConfigRegex(rName string) string {
	return fmt.Sprintf(`
# Create an agent for testing
resource "archestra_agent" "regex" {
  name = "tip-regex-agent-%[1]s"
}

# Create an MCP server in the registry
resource "archestra_mcp_server" "regex" {
  name        = "tip-regex-server-%[1]s"
  description = "MCP server for regex testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Install the MCP server
resource "archestra_mcp_server_installation" "regex" {
  name          = "tip-regex-install-%[1]s"
  mcp_server_id = archestra_mcp_server.regex.id
}

# Look up the agent tool
data "archestra_agent_tool" "regex" {
  agent_id  = archestra_agent.regex.id
  tool_name = "archestra__whoami"

  depends_on = [archestra_mcp_server_installation.regex]
}

# Create a tool invocation policy with regex
resource "archestra_tool_invocation_policy" "regex" {
  agent_tool_id = data.archestra_agent_tool.regex.id
  argument_name = "path"
  operator      = "regex"
  value         = "^/home/[a-z]+/.ssh/.*"
  action        = "block_always"
  reason        = "Block SSH key access using regex pattern"
}
`, rName)
}
