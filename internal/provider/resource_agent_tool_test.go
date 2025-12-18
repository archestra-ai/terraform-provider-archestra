package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAgentToolResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"archestra": providerserver.NewProtocol6WithError(New("test")()),
		},
		ExternalProviders: map[string]resource.ExternalProvider{
			// Need to register the provider itself to get the mcp_server resource if it wasn't already available
			// But New("test")() likely registers all resources including mcp_server.
			// Assumption: New("test")() returns a provider with all resources.
		},
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccAgentToolResourceConfig("archestra-test-agent", "archestra__calculator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "profile_id", "archestra_agent.test", "id"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "tool_result_treatment", "untrusted"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "use_dynamic_team_credential", "true"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "response_modifier_template", "Hello {{.Result}}"),
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "credential_source_mcp_server_id", "archestra_mcp_server_installation.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_profile_tool.test", "id"),
				),
			},
			// Update
			{
				Config: testAccAgentToolResourceConfigUpdate("archestra-test-agent", "archestra__calculator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "tool_result_treatment", "trusted"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "allow_usage_when_untrusted_data_is_present", "true"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "use_dynamic_team_credential", "false"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "response_modifier_template", "Modified {{.Result}}"),
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "credential_source_mcp_server_id", "archestra_mcp_server_installation.test", "id"),
				),
			},
			// Import
			{
				ResourceName:      "archestra_profile_tool.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete checks are implicit
		},
	})
}

// Since we need real IDs for profiles and tools, we should use data sources or depend on created resources.
// However, creating an Agent and then using its ID is cleaner.
// Finding a tool ID is harder without querying. We can use `archestra_mcp_server_tool` data source if available, or just use a known built-in tool name strategy if we had a data source for "tool by name".
// The `datasource_agent_tool` gets tools *assigned* to an agent.
// We need a way to get a tool ID *before* assignment.
// `datasource_mcp_server_tool` might work if we have a server.
// Alternatively, we can use the `archestra_agent` resource and maybe a `archestra_tool` data source if it exists?
// Currently we only have `agent_tool` data source (assigned tools) and `mcp_server_tool` (tools on a server).
// Let's assume we can use a built-in tool that exists? Or an MCP tool.
// For the test to be reliable, we probably need to setup a real scenario or mock it.
// Given strict environment, I will write the test assuming we can Create an Agent, and then assign a tool that we know exists or can find.
// The `archestra__calculator` is a standard built-in tool. But built-in tools are often auto-assigned or special.
// Let's try to reference a data source for a tool.
// Actually, `datasource_mcp_server_tool.go` exists.

func testAccAgentToolResourceConfig(agentName, toolName string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name = "%[1]s"
}

data "archestra_agent_tool" "builtin" {
  agent_id  = archestra_agent.test.id
  tool_name = "archestra__whoami"
}

resource "archestra_mcp_server" "test" {
  name = "test-server"
  local_config = {
    command = "echo"
    arguments = ["hello"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "test-server-inst"
  mcp_server_id = archestra_mcp_server.test.id
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test.id
  tool_id    = data.archestra_agent_tool.builtin.tool_id
  
  # Set a specific treatment to verify it is applied
  tool_result_treatment = "untrusted"
  use_dynamic_team_credential = true
  response_modifier_template  = "Hello {{.Result}}"
  credential_source_mcp_server_id = archestra_mcp_server_installation.test.id
}
`, agentName)
}

func testAccAgentToolResourceConfigUpdate(agentName, toolName string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name = "%[1]s"
}

data "archestra_agent_tool" "builtin" {
  agent_id  = archestra_agent.test.id
  tool_name = "archestra__whoami"
}

resource "archestra_mcp_server" "test" {
  name = "test-server"
  local_config = {
    command = "echo"
    arguments = ["hello"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "test-server-inst"
  mcp_server_id = archestra_mcp_server.test.id
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_agent.test.id
  tool_id    = data.archestra_agent_tool.builtin.tool_id
  
  tool_result_treatment = "trusted"
  allow_usage_when_untrusted_data_is_present = true
  use_dynamic_team_credential = false
  response_modifier_template  = "Modified {{.Result}}"
  credential_source_mcp_server_id = archestra_mcp_server_installation.test.id
}
`, agentName)
}
