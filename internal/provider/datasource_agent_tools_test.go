package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAgentToolsDataSource verifies the plural data source returns
// the live assignment list for a given agent and that each entry
// carries the per-assignment fields (assignment_id, tool_id,
// mcp_server_id, credential_resolution_mode).
func TestAccAgentToolsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentToolsDataSourceConfig("tf-acc-at-ds"),
				ConfigStateChecks: []statecheck.StateCheck{
					// At least one assignment came back.
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tools.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("tool_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tools.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("assignment_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tools.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("credential_resolution_mode"),
						knownvalue.StringExact("static"),
					),
				},
			},
		},
	})
}

func testAccAgentToolsDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name        = %[1]q
  description = "Acceptance-test agent for data.archestra_agent_tools"
  scope       = "org"
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

# Bulk-assign the install's tools to drive the assignment list the
# data source then queries.
resource "archestra_agent_tool_batch" "test" {
  agent_id      = archestra_agent.test.id
  mcp_server_id = archestra_mcp_server_installation.test.id
  tool_ids      = toset([for t in archestra_mcp_server_installation.test.tools : t.id])
}

data "archestra_agent_tools" "test" {
  agent_id   = archestra_agent.test.id
  depends_on = [archestra_agent_tool_batch.test]
}
`, name)
}
