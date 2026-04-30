package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAgentToolResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentToolResourceConfig("tf-acc-agent-tool"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent_tool.test", tfjsonpath.New("credential_resolution_mode"), knownvalue.StringExact("static")),
				},
			},
			// Bug 11 round-trip pin — id format is `<agent_id>:<tool_id>`,
			// Read writes both back to state so the next plan after import
			// doesn't diff agent_id/tool_id and trigger destroy+recreate.
			{
				ResourceName:      "archestra_agent_tool.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAgentToolResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_gateway" "test" {
  name = %q
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "%s-server"
  description = "Test MCP server"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name       = "%s-install"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server_installation.test.id
  name          = "${archestra_mcp_registry_catalog_item.test.name}__read_text_file"
}

resource "archestra_agent_tool" "test" {
  agent_id      = archestra_mcp_gateway.test.id
  tool_id       = data.archestra_mcp_server_tool.test.id
  mcp_server_id = archestra_mcp_server_installation.test.id
}
`, name, name, name)
}
