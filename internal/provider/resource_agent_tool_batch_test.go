package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccAgentToolBatchResource exercises the bulk-assign happy path and
// the in-place update behaviour that the review-driven cleanup pinned:
//
//   - Create: every tool from a fresh filesystem MCP install assigned to
//     a new agent in one round-trip.
//   - Update: shrinking `tool_ids` (here: take the first half of the
//     install's tools) plans as `update in-place`, NOT `replace`. This
//     is the behaviour the schema's "asymmetric replacement" callout
//     promises and the U2 review fix documented.
//   - Import: round-trips through `<agent_id>:<mcp_server_id>`.
func TestAccAgentToolBatchResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create: assign every tool from the install.
				Config: testAccAgentToolBatchResourceConfig("tf-acc-bulk", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_agent_tool_batch.test",
						tfjsonpath.New("credential_resolution_mode"),
						knownvalue.StringExact("static"),
					),
					statecheck.ExpectKnownValue(
						"archestra_agent_tool_batch.test",
						tfjsonpath.New("tool_ids"),
						knownvalue.NotNull(),
					),
				},
			},
			{
				// Update: shrink to the first half. Verifies the
				// in-place update path (NoOp at plan stage if the
				// resource is going to be replaced; Update at the
				// resource if it's a true in-place change).
				Config: testAccAgentToolBatchResourceConfig("tf-acc-bulk", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"archestra_agent_tool_batch.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
			{
				ResourceName:                         "archestra_agent_tool_batch.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
				// `tool_ids` is read live so it round-trips; we don't
				// pre-seed `credential_resolution_mode` from the import
				// ID so the framework reconstructs it from defaults.
				ImportStateVerifyIgnore: []string{"credential_resolution_mode"},
			},
		},
	})
}

func testAccAgentToolBatchResourceConfig(name string, allTools bool) string {
	// Pick all tools on Create, the first 5 on Update. The filesystem
	// server advertises ~14 tools; first-5 is well under that, so the
	// shrink case exercises the "remove" path of the diff.
	toolExpr := `toset([for t in archestra_mcp_server_installation.test.tools : t.id])`
	if !allTools {
		toolExpr = `toset(slice([for t in archestra_mcp_server_installation.test.tools : t.id], 0, 5))`
	}
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name        = %[1]q
  description = "Acceptance-test agent for archestra_agent_tool_batch"
  scope       = "org"
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "%[1]s-cat"
  description = "Bulk-assign acceptance test catalog item"

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
  tool_ids      = %[2]s
}
`, name, toolExpr)
}
