package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccToolInvocationPolicyDefaultResource pins three properties of
// the bulk-default invocation-policy resource that the review surfaced
// (C1 / C4):
//
//  1. `id` is fixed-length (77 chars: `<action>:<sha256-hex>`) regardless
//     of the size of `tool_ids` — replaces the old comma-joined-UUID
//     scheme that grew unboundedly.
//  2. Changing `action` updates in-place — does NOT trigger replacement —
//     and the `id` stays the same across the change.
//  3. Changing `tool_ids` updates in-place and the `id` stays the same.
//
// Both invariants are necessary because Terraform requires the `id`
// attribute to be stable across a resource's lifetime.
func TestAccToolInvocationPolicyDefaultResource(t *testing.T) {
	var capturedID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolInvocationPolicyDefaultConfig("tf-acc-tipd", true, "block_always"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy_default.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
					// 77 chars: "block_always:" (13) + 64-hex SHA-256.
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy_default.test",
						tfjsonpath.New("id"),
						knownvalue.StringRegexp(regexp.MustCompile(`^block_always:[0-9a-f]{64}$`)),
					),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					captureAttr("archestra_tool_invocation_policy_default.test", "id", &capturedID),
				),
			},
			{
				// Update action: in-place, ID stable.
				Config: testAccToolInvocationPolicyDefaultConfig("tf-acc-tipd", true, "require_approval"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"archestra_tool_invocation_policy_default.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					assertAttrEquals("archestra_tool_invocation_policy_default.test", "id", &capturedID),
				),
			},
			{
				// Update tool_ids: in-place, ID still stable.
				Config: testAccToolInvocationPolicyDefaultConfig("tf-acc-tipd", false, "require_approval"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"archestra_tool_invocation_policy_default.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					assertAttrEquals("archestra_tool_invocation_policy_default.test", "id", &capturedID),
				),
			},
			{
				// Import: use the action name as the ID; Read reconciles
				// tool_ids from the live policies table.
				ResourceName:      "archestra_tool_invocation_policy_default.test",
				ImportState:       true,
				ImportStateId:     "require_approval",
				ImportStateVerify: true,
			},
		},
	})
}

// testAccToolInvocationPolicyDefaultConfig builds an HCL config that
// installs a filesystem MCP server (so we have real tool UUIDs) and
// applies a bulk-default invocation policy across either the full tool
// set or the first three.
func testAccToolInvocationPolicyDefaultConfig(name string, allTools bool, action string) string {
	toolExpr := `toset([for t in archestra_mcp_server_installation.test.tools : t.id])`
	if !allTools {
		toolExpr = `toset(slice([for t in archestra_mcp_server_installation.test.tools : t.id], 0, 3))`
	}
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "%[1]s-cat"
  description = "Bulk default-invocation-policy acceptance test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name       = "%[1]s-install"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}

resource "archestra_tool_invocation_policy_default" "test" {
  tool_ids = %[2]s
  action   = %[3]q
}
`, name, toolExpr, action)
}
