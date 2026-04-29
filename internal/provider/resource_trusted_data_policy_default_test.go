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

// TestAccTrustedDataPolicyDefaultResource pins the same id-stability
// invariants as the invocation-policy default test (action change +
// tool_ids change both update in-place, id stays the same), against
// the trusted-data action enum.
func TestAccTrustedDataPolicyDefaultResource(t *testing.T) {
	var capturedID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTrustedDataPolicyDefaultConfig("tf-acc-tdpd", true, "mark_as_trusted"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy_default.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("mark_as_trusted"),
					),
					statecheck.ExpectKnownValue(
						"archestra_trusted_data_policy_default.test",
						tfjsonpath.New("id"),
						knownvalue.StringRegexp(regexp.MustCompile(`^mark_as_trusted:[0-9a-f]{64}$`)),
					),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					captureAttr("archestra_trusted_data_policy_default.test", "id", &capturedID),
				),
			},
			{
				Config: testAccTrustedDataPolicyDefaultConfig("tf-acc-tdpd", true, "sanitize_with_dual_llm"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"archestra_trusted_data_policy_default.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					assertAttrEquals("archestra_trusted_data_policy_default.test", "id", &capturedID),
				),
			},
			{
				Config: testAccTrustedDataPolicyDefaultConfig("tf-acc-tdpd", false, "sanitize_with_dual_llm"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"archestra_trusted_data_policy_default.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					assertAttrEquals("archestra_trusted_data_policy_default.test", "id", &capturedID),
				),
			},
		},
	})
}

func testAccTrustedDataPolicyDefaultConfig(name string, allTools bool, action string) string {
	toolExpr := `toset([for t in archestra_mcp_server_installation.test.tools : t.id])`
	if !allTools {
		toolExpr = `toset(slice([for t in archestra_mcp_server_installation.test.tools : t.id], 0, 3))`
	}
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "%[1]s-cat"
  description = "Bulk default-trusted-data-policy acceptance test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name       = "%[1]s-install"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}

resource "archestra_trusted_data_policy_default" "test" {
  tool_ids = %[2]s
  action   = %[3]q
}
`, name, toolExpr, action)
}
