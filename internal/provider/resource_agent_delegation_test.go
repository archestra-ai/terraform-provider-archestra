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

// TestAccAgentDelegationResource exercises the full Create → Read →
// Update → Re-apply (no-op) → ImportState → Delete lifecycle. Update
// path is the load-bearing one: the backend's sync endpoint replaces
// the entire delegation set, so changing target_agent_ids must NOT
// require replacement of the resource itself.
func TestAccAgentDelegationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentDelegationConfig(`[archestra_agent.target_a.id]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_agent_delegation.test",
						tfjsonpath.New("target_agent_ids"),
						knownvalue.SetSizeExact(1),
					),
				},
			},
			// Idempotent re-apply — pins the Read path against the
			// sorted-set wire order. Without it, a non-deterministic
			// backend order would surface here as plan diff.
			{
				Config: testAccAgentDelegationConfig(`[archestra_agent.target_a.id]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Update: add a second target — must NOT trigger replacement.
			{
				Config: testAccAgentDelegationConfig(`[archestra_agent.target_a.id, archestra_agent.target_b.id]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_agent_delegation.test",
						tfjsonpath.New("target_agent_ids"),
						knownvalue.SetSizeExact(2),
					),
				},
			},
			// Update: drop one target.
			{
				Config: testAccAgentDelegationConfig(`[archestra_agent.target_b.id]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_agent_delegation.test",
						tfjsonpath.New("target_agent_ids"),
						knownvalue.SetSizeExact(1),
					),
				},
			},
			// ImportState — bare source-agent-id.
			{
				ResourceName:      "archestra_agent_delegation.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccAgentDelegationResource_LLMProxyRejected pins the backend's
// 400 on llm_proxy delegation. The provider relays the error verbatim
// instead of pre-empting in ValidateConfig — agent_type isn't known
// until Read of the source agent, so a plan-time validator can't see
// it without an extra round-trip.
func TestAccAgentDelegationResource_LLMProxyRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_llm_proxy" "proxy" {
  name = "tf-acc-delegation-proxy"
}

resource "archestra_agent" "target" {
  name = "tf-acc-delegation-target"
}

resource "archestra_agent_delegation" "bad" {
  agent_id         = archestra_llm_proxy.proxy.id
  target_agent_ids = [archestra_agent.target.id]
}
`,
				ExpectError: regexp.MustCompile(`(?i)LLM proxies cannot have subagents|400`),
			},
		},
	})
}

func testAccAgentDelegationConfig(targetExpr string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "source" {
  name = "tf-acc-delegation-source"
}

resource "archestra_agent" "target_a" {
  name = "tf-acc-delegation-target-a"
}

resource "archestra_agent" "target_b" {
  name = "tf-acc-delegation-target-b"
}

resource "archestra_agent_delegation" "test" {
  agent_id         = archestra_agent.source.id
  target_agent_ids = %s
}
`, targetExpr)
}
