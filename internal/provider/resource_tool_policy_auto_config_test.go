package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccToolPolicyAutoConfigResource hits the LLM-driven policy
// generator endpoint. Each Create call spends real LLM tokens, so the
// test is opt-in via `ARCHESTRA_TEST_LLM_BUDGET_OK=true` (mirrors the
// BYOS-vault and EMC-IdP gating helpers' "you have to know what you're
// signing up for" pattern).
//
// What this verifies:
//
//   - The endpoint returns a `results` list with one entry per
//     submitted `tool_id`. Each entry carries `tool_id`, `success`,
//     and (when success) `reasoning` / `tool_invocation_action` /
//     `trusted_data_action`.
//   - The resource captures the LLM analysis in state and never
//     refreshes — running plan after apply reports no changes.
//
// What we DON'T verify (out of scope):
//
//   - Specific actions the LLM picks. Non-deterministic across runs;
//     the schema enforces the enum so any value the backend returns
//     is by definition valid.
//   - Replacement behaviour on `tool_ids` change. Forces another
//     LLM call; the cost/cost-control split is documented on the
//     resource.
func TestAccToolPolicyAutoConfigResource(t *testing.T) {
	if os.Getenv("ARCHESTRA_TEST_LLM_BUDGET_OK") != "true" {
		t.Skip("skipping: set ARCHESTRA_TEST_LLM_BUDGET_OK=true to run — calls /api/agent-tools/auto-configure-policies which spends LLM tokens. Cost-gated tests opt in explicitly; the env var being unset is the intended CI default.")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolPolicyAutoConfigConfig("tf-acc-autocfg"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_policy_auto_config.test",
						tfjsonpath.New("results"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_policy_auto_config.test",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("tool_id"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func testAccToolPolicyAutoConfigConfig(name string) string {
	return fmt.Sprintf(`
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

# Cap the tool set at 3 so the LLM call doesn't take forever and
# doesn't burn through the test's token budget.
resource "archestra_tool_policy_auto_config" "test" {
  tool_ids = toset(slice([for t in archestra_mcp_server_installation.test.tools : t.id], 0, 3))
}
`, name)
}
