package provider

import (
	"fmt"
	"regexp"
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
						tfjsonpath.New("conditions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"key":      knownvalue.StringExact("path"),
								"operator": knownvalue.StringExact("contains"),
								"value":    knownvalue.StringExact("/etc/"),
							}),
						}),
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
						tfjsonpath.New("conditions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"key":      knownvalue.StringExact("path"),
								"operator": knownvalue.StringExact("startsWith"),
								"value":    knownvalue.StringExact("/var/log/"),
							}),
						}),
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
						tfjsonpath.New("conditions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"key":      knownvalue.StringExact("command"),
								"operator": knownvalue.StringExact("equal"),
								"value":    knownvalue.StringExact("rm -rf"),
							}),
						}),
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
resource "archestra_mcp_gateway" "noreason" {
  name = "tip-noreason-profile-%[1]s"
}

resource "archestra_mcp_registry_catalog_item" "noreason" {
  name = "tip-noreason-server-%[1]s"
  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "noreason" {
  name          = "tip-noreason-inst-%[1]s"
  catalog_id = archestra_mcp_registry_catalog_item.noreason.id
}

data "archestra_mcp_server_tool" "noreason" {
  mcp_server_id = archestra_mcp_server_installation.noreason.id
  name          = "${archestra_mcp_registry_catalog_item.noreason.name}__list_directory"
}

resource "archestra_agent_tool" "noreason" {
  agent_id    = archestra_mcp_gateway.noreason.id
  tool_id       = data.archestra_mcp_server_tool.noreason.id
  mcp_server_id = archestra_mcp_server_installation.noreason.id
}

data "archestra_agent_tool" "noreason" {
  agent_id = archestra_mcp_gateway.noreason.id
  tool_name  = "${archestra_mcp_registry_catalog_item.noreason.name}__list_directory"
  depends_on = [archestra_agent_tool.noreason]
}

resource "archestra_tool_invocation_policy" "noreason" {
  tool_id = data.archestra_mcp_server_tool.noreason.id
  conditions = [
    { key = "command", operator = "equal", value = "rm -rf" },
  ]
  action = "block_always"
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
						tfjsonpath.New("conditions"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"key":      knownvalue.StringExact("path"),
								"operator": knownvalue.StringExact("regex"),
								"value":    knownvalue.StringExact("^/home/[a-z]+/.ssh/.*"),
							}),
						}),
					),
				},
			},
		},
	})
}

func TestAccToolInvocationPolicyResource_InvalidToolID(t *testing.T) {
	// Pure plan-time schema validation; does not hit the backend.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_tool_invocation_policy" "invalid" {
  tool_id = "not-a-uuid"
  conditions = [
    { key = "path", operator = "contains", value = "/etc/" },
  ]
  action = "block_always"
}
`,
				ExpectError: regexp.MustCompile(`tool_id must be a UUID`),
			},
		},
	})
}

func testAccToolInvocationPolicyResourceConfig(rName string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_gateway" "test" {
  name = "tip-test-profile-%[1]s"
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name = "tip-test-server-%[1]s"
  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "tip-test-inst-%[1]s"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server_installation.test.id
  name          = "${archestra_mcp_registry_catalog_item.test.name}__list_directory"
}

resource "archestra_agent_tool" "test" {
  agent_id    = archestra_mcp_gateway.test.id
  tool_id       = data.archestra_mcp_server_tool.test.id
  mcp_server_id = archestra_mcp_server_installation.test.id
}

data "archestra_agent_tool" "test" {
  agent_id = archestra_mcp_gateway.test.id
  tool_name  = "${archestra_mcp_registry_catalog_item.test.name}__list_directory"
  depends_on = [archestra_agent_tool.test]
}

resource "archestra_tool_invocation_policy" "test" {
  tool_id = data.archestra_mcp_server_tool.test.id
  conditions = [
    { key = "path", operator = "contains", value = "/etc/" },
  ]
  action = "block_always"
  reason = "Block access to system configuration files"
}
`, rName)
}

func testAccToolInvocationPolicyResourceConfigUpdated(rName string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_gateway" "test" {
  name = "tip-test-profile-%[1]s"
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name = "tip-test-server-%[1]s"
  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "tip-test-inst-%[1]s"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server_installation.test.id
  name          = "${archestra_mcp_registry_catalog_item.test.name}__list_directory"
}

resource "archestra_agent_tool" "test" {
  agent_id    = archestra_mcp_gateway.test.id
  tool_id       = data.archestra_mcp_server_tool.test.id
  mcp_server_id = archestra_mcp_server_installation.test.id
}

data "archestra_agent_tool" "test" {
  agent_id = archestra_mcp_gateway.test.id
  tool_name  = "${archestra_mcp_registry_catalog_item.test.name}__list_directory"
  depends_on = [archestra_agent_tool.test]
}

resource "archestra_tool_invocation_policy" "test" {
  tool_id = data.archestra_mcp_server_tool.test.id
  conditions = [
    { key = "path", operator = "startsWith", value = "/var/log/" },
  ]
  action = "allow_when_context_is_untrusted"
  reason = "Allow log file access in untrusted contexts"
}
`, rName)
}

func testAccToolInvocationPolicyResourceConfigRegex(rName string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_gateway" "regex" {
  name = "tip-regex-profile-%[1]s"
}

resource "archestra_mcp_registry_catalog_item" "regex" {
  name = "tip-regex-server-%[1]s"
  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "regex" {
  name          = "tip-regex-inst-%[1]s"
  catalog_id = archestra_mcp_registry_catalog_item.regex.id
}

data "archestra_mcp_server_tool" "regex" {
  mcp_server_id = archestra_mcp_server_installation.regex.id
  name          = "${archestra_mcp_registry_catalog_item.regex.name}__list_directory"
}

resource "archestra_agent_tool" "regex" {
  agent_id    = archestra_mcp_gateway.regex.id
  tool_id       = data.archestra_mcp_server_tool.regex.id
  mcp_server_id = archestra_mcp_server_installation.regex.id
}

data "archestra_agent_tool" "regex" {
  agent_id = archestra_mcp_gateway.regex.id
  tool_name  = "${archestra_mcp_registry_catalog_item.regex.name}__list_directory"
  depends_on = [archestra_agent_tool.regex]
}

resource "archestra_tool_invocation_policy" "regex" {
  tool_id = data.archestra_mcp_server_tool.regex.id
  conditions = [
    { key = "path", operator = "regex", value = "^/home/[a-z]+/.ssh/.*" },
  ]
  action = "block_always"
  reason = "Block SSH key access using regex pattern"
}
`, rName)
}
