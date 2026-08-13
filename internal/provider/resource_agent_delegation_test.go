package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAgentDelegationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentDelegationResourceConfig("tf-acc-agent-delegation"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValuePairs(
						"archestra_agent_delegation.test", tfjsonpath.New("agent_id"),
						"archestra_agent.delegator", tfjsonpath.New("id"),
						compare.ValuesSame(),
					),
					statecheck.CompareValuePairs(
						"archestra_agent_delegation.test", tfjsonpath.New("target_agent_id"),
						"archestra_agent.delegate", tfjsonpath.New("id"),
						compare.ValuesSame(),
					),
				},
			},
			// The id is `<agent_id>:<target_agent_id>`; Read writes both halves
			// back to state so the plan after an import doesn't diff the
			// Required+RequiresReplace attributes and trigger destroy+recreate.
			{
				ResourceName:      "archestra_agent_delegation.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Exercises the read-union-sync path: adding a second edge from the
			// same delegating agent preserves the first (sequential apply — this
			// does not exercise parallel-create racing).
			{
				Config: testAccAgentDelegationResourceConfigTwoEdges("tf-acc-agent-delegation"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent_delegation.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("archestra_agent_delegation.second", tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func testAccAgentDelegationResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "delegator" {
  name = "%s-delegator"
}

resource "archestra_agent" "delegate" {
  name = "%s-delegate"
}

resource "archestra_agent_delegation" "test" {
  agent_id        = archestra_agent.delegator.id
  target_agent_id = archestra_agent.delegate.id
}
`, name, name)
}

func testAccAgentDelegationResourceConfigTwoEdges(name string) string {
	return testAccAgentDelegationResourceConfig(name) + fmt.Sprintf(`
resource "archestra_agent" "second_delegate" {
  name = "%s-delegate-2"
}

resource "archestra_agent_delegation" "second" {
  agent_id        = archestra_agent.delegator.id
  target_agent_id = archestra_agent.second_delegate.id
}
`, name)
}
