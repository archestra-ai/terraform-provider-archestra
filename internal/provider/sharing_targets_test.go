package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// A team, its service account, a model pool, a virtual key shared with both
// and limits on the team and the key are created in one apply, each naming
// the others by reference instead of by subject type and ID.
func TestAccSharingAndLimitTargets(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-targets")
	poolResource := ""
	config := func(teamLimit, poolIDs string) string {
		return fmt.Sprintf(`
resource "archestra_team" "a" { name = "%[1]s-a" }
resource "archestra_team" "b" { name = "%[1]s-b" }

resource "archestra_service_account" "test" {
  name = "%[1]s-sa"
  role = "member"
}

resource "archestra_model_provider" "test" {
  name         = "%[1]s-provider"
  llm_provider = "ollama"
  %[4]s
  base_url     = %[5]q
  shared       = true
}

%[6]s

resource "archestra_virtual_key" "test" {
  name           = "%[1]s-key"
  %[3]s
  permissions = [
    { team_id = archestra_team.a.id, actions = ["read"] },
    { service_account_id = archestra_service_account.test.id, actions = ["read", "use"] },
  ]
}

resource "archestra_limit" "team" {
  %[2]s
  limit_type  = "token_cost"
  limit_value = 100
}

# A key's cost budget is its spend_cap; a limit adds a quota on calls to
# one MCP server.
resource "archestra_limit" "key" {
  virtual_key_id  = archestra_virtual_key.test.id
  limit_type      = "mcp_server_calls"
  limit_value     = 50
  mcp_server_name = "tf-acc-server"
}
`, name, teamLimit, poolIDs, testAccLLMKeyMaterial(), os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL"), poolResource)
	}
	// Virtual keys take model pools from 1.5.0-rc.12.
	withPools := os.Getenv("TF_ACC") != "" && testAccAPIVersionAtLeast(t, "1.5.0-rc.12")
	pools := `model_pool_ids = [archestra_model_pool.test.id]`
	noPools := "model_pool_ids = []\n  include_future_providers = true"
	poolChecks := resource.TestCheckTypeSetElemAttrPair("archestra_virtual_key.test", "model_pool_ids.*", "archestra_model_pool.test", "id")
	if withPools {
		poolResource = fmt.Sprintf(`resource "archestra_model_pool" "test" {
  name    = "%s-pool"
  members = [{ provider_api_key_id = archestra_model_provider.test.id, model_id = "tf-acc-model" }]
}`, name)
	} else {
		pools, noPools = "include_future_providers = true", "include_future_providers = true"
		poolChecks = resource.TestCheckNoResourceAttr("archestra_virtual_key.test", "model_pool_ids")
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL") == "" {
				t.Fatal("ARCHESTRA_TEST_LLM_BASE_URL must point to the isolated model fixture")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(`team_id = archestra_team.a.id`, pools),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_limit.team", "entity_type", "team"),
					resource.TestCheckResourceAttrPair("archestra_limit.team", "entity_id", "archestra_team.a", "id"),
					resource.TestCheckResourceAttr("archestra_limit.key", "entity_type", "virtual_key"),
					resource.TestCheckResourceAttrPair("archestra_limit.key", "entity_id", "archestra_virtual_key.test", "id"),
					resource.TestCheckNoResourceAttr("archestra_limit.key", "team_id"),
					poolChecks,
					resource.TestCheckTypeSetElemNestedAttrs("archestra_virtual_key.test", "permissions.*", map[string]string{"subject_type": "team", "actions.#": "1"}),
					resource.TestCheckTypeSetElemNestedAttrs("archestra_virtual_key.test", "permissions.*", map[string]string{"subject_type": "serviceAccount", "actions.#": "2"}),
				),
			},
			{Config: config(`team_id = archestra_team.a.id`, pools), PlanOnly: true},
			// The generic form of the same limit plans no change.
			{Config: config(`entity_type = "team"`+"\n"+`  entity_id = archestra_team.a.id`, pools), PlanOnly: true},
			{
				Config: config(`team_id = archestra_team.b.id`, pools),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("archestra_limit.team", plancheck.ResourceActionReplace),
					plancheck.ExpectResourceAction("archestra_limit.key", plancheck.ResourceActionNoop),
				}},
				Check: resource.TestCheckResourceAttrPair("archestra_limit.team", "team_id", "archestra_team.b", "id"),
			},
			{ResourceName: "archestra_limit.team", ImportState: true, ImportStateVerify: true},
			{
				Config: config(`team_id = archestra_team.b.id`, noPools),
				Check: func(s *terraform.State) error {
					if !withPools {
						return nil
					}
					return resource.TestCheckResourceAttr("archestra_virtual_key.test", "model_pool_ids.#", "0")(s)
				},
			},
			{
				Config:      config(`team_id = archestra_team.b.id`+"\n"+`  entity_type = "team"`, noPools),
				ExpectError: regexp.MustCompile(`Invalid Limit Target`),
			},
			{
				Config: config(`team_id = archestra_team.b.id`, noPools) + `
resource "archestra_project" "both" {
  name        = "tf-acc-both-forms"
  permissions = [{ team_id = archestra_team.a.id, subject_type = "team", subject_id = archestra_team.a.id, actions = ["read"] }]
}`,
				ExpectError: regexp.MustCompile(`Invalid Grant Principal`),
			},
			// A valid configuration last, so the final destroy can plan.
			{Config: config(`team_id = archestra_team.b.id`, noPools), PlanOnly: true},
		},
	})
}
