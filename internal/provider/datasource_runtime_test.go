package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAgentEmailAddressDataSource_NotFound pins the 404 path on the
// singular lookup. Positive-path coverage requires the org-level email
// provider to be enabled, which the local test stack doesn't run; the
// 404 case is the more reliable regression pin.
func TestAccAgentEmailAddressDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "archestra_agent_email_address" "missing" { agent_id = "00000000-0000-0000-0000-000000000000" }`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

// TestAccAgentLabelKeysDataSource asserts the list endpoint returns 200
// even when no agents have labels (empty list is valid).
func TestAccAgentLabelKeysDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_agent_label_keys" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_agent_label_keys.all", "keys.#"),
				),
			},
		},
	})
}

// TestAccAgentLabelValuesDataSource exercises the unfiltered list path.
// Filtering by `key` is exercised against a real key in the agent-tagged
// integration tests; pinning that here would require provisioning a
// label-bearing agent inside this test, which is unrelated to the
// data source's contract.
func TestAccAgentLabelValuesDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_agent_label_values" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_agent_label_values.all", "values.#"),
				),
			},
		},
	})
}

func TestAccMCPCatalogLabelKeysDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_mcp_catalog_label_keys" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_mcp_catalog_label_keys.all", "keys.#"),
				),
			},
		},
	})
}

func TestAccMCPCatalogLabelValuesDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_mcp_catalog_label_values" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_mcp_catalog_label_values.all", "values.#"),
				),
			},
		},
	})
}

// TestAccAutonomyPolicyOperatorsDataSource asserts the operators list
// is non-empty — the backend always advertises at least equal/notEqual.
func TestAccAutonomyPolicyOperatorsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_autonomy_policy_operators" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("data.archestra_autonomy_policy_operators.all", "operators.#", regexp.MustCompile(`^[1-9]`)),
					resource.TestCheckResourceAttrSet("data.archestra_autonomy_policy_operators.all", "operators.0.value"),
					resource.TestCheckResourceAttrSet("data.archestra_autonomy_policy_operators.all", "operators.0.label"),
				),
			},
		},
	})
}
