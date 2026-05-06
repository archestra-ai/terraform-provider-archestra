package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAgentToolDataSource_NamespacedToolName pins the `tool_name`
// contract: it expects the backend's slugified `<prefix>__<raw>` form,
// not the bare tool name. A previously buggy example that passed
// `tool_name = "read_text_file"` failed with "Tool 'read_text_file'
// not found" because the backend stores it as
// `<catalog-item-name>__read_text_file`. This test pins the corrected
// form so the schema/example doc and the runtime expectation stay in
// sync.
func TestAccAgentToolDataSource_NamespacedToolName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentToolDataSourceNamespacedConfig("tf-acc-at-ds-ns"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tool.test",
						tfjsonpath.New("tool_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tool.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func testAccAgentToolDataSourceNamespacedConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name  = %[1]q
  scope = "org"
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name = "%[1]s-cat"
  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name       = "%[1]s-install"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}

resource "archestra_agent_tool_batch" "test" {
  agent_id      = archestra_agent.test.id
  mcp_server_id = archestra_mcp_server_installation.test.id
  tool_ids      = toset([for t in archestra_mcp_server_installation.test.tools : t.id])
}

data "archestra_agent_tool" "test" {
  agent_id   = archestra_agent.test.id
  tool_name  = "${archestra_mcp_registry_catalog_item.test.name}__read_text_file"
  depends_on = [archestra_agent_tool_batch.test]
}
`, name)
}
