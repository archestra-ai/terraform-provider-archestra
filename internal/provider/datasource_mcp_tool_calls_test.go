package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccMcpToolCallsDataSource exercises the audit-log data source on
// an empty backend (the local stack starts with no recorded tool
// calls). Verifies:
//
//   - the data source reads cleanly and returns total=0 / truncated=false
//     when no calls match;
//   - the `max_records` cap is honoured even on empty input — i.e. the
//     paging loop terminates correctly without pulling beyond the cap.
//
// Populated-backend behaviour (real call records, search filtering,
// truncation flipping to true) is left for a future test that seeds the
// backend with synthetic calls; the local stack has no fixture for that
// today.
func TestAccMcpToolCallsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "archestra_mcp_tool_calls" "empty" {}

data "archestra_mcp_tool_calls" "capped" {
  max_records = 5
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_tool_calls.empty",
						tfjsonpath.New("total"),
						knownvalue.Int64Exact(0),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_tool_calls.empty",
						tfjsonpath.New("truncated"),
						knownvalue.Bool(false),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_tool_calls.capped",
						tfjsonpath.New("total"),
						knownvalue.Int64Exact(0),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_tool_calls.capped",
						tfjsonpath.New("truncated"),
						knownvalue.Bool(false),
					),
				},
			},
		},
	})
}
