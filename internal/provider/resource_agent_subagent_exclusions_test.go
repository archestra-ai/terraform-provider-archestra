package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccAgentSubagentExclusionsResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-subagent-exclusions")
	var agentID string
	base := fmt.Sprintf(`
resource "archestra_agent" "parent" {
 name=%q
 system_prompt="Synthetic delegation fixture."
 access_all_subagents=true
}
resource "archestra_agent" "first" {
 name=%q
 system_prompt="First synthetic target."
}
resource "archestra_agent" "second" {
 name=%q
 system_prompt="Second synthetic target."
}
`, name, name+"-first", name+"-second")
	config := func(ids string) string {
		return base + fmt.Sprintf(`
resource "archestra_agent_subagent_exclusions" "test" {
 agent_id=archestra_agent.parent.id
 excluded_subagent_ids=%s
}
`, ids)
	}
	initial, updated := config("[archestra_agent.first.id]"), config("[archestra_agent.second.id]")
	apiClient := func() *client.ClientWithResponses {
		c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_agent_subagent_exclusions.test", "excluded_subagent_ids.#", "1"), func(s *terraform.State) error {
			agentID = s.RootModule().Resources["archestra_agent.parent"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_agent_subagent_exclusions.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttrPair("archestra_agent_subagent_exclusions.test", "excluded_subagent_ids.0", "archestra_agent.second", "id")},
		{Config: updated, PlanOnly: true},
		{Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
			response, err := apiClient().UpdateAgentSubagentExclusionsWithResponse(t.Context(), uuid.MustParse(agentID), client.UpdateAgentSubagentExclusionsJSONRequestBody{ExcludedSubagentIds: []uuid.UUID{}})
			if err != nil || response.JSON200 == nil {
				t.Fatal("Unable to write isolated exclusion drift", err)
			}
		}},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_agent_subagent_exclusions.test", "excluded_subagent_ids.#", "1")},
		{Config: config("[]"), Check: resource.TestCheckResourceAttr("archestra_agent_subagent_exclusions.test", "excluded_subagent_ids.#", "0")},
		{Config: updated},
		{Config: base, Check: func(_ *terraform.State) error {
			response, err := apiClient().GetAgentSubagentExclusionsWithResponse(t.Context(), uuid.MustParse(agentID))
			if err != nil || response.JSON200 == nil {
				return fmt.Errorf("deletion removed parent agent: %w", err)
			}
			if len(response.JSON200.ExcludedSubagentIds) != 0 {
				return fmt.Errorf("deletion did not clear exclusions")
			}
			return nil
		}},
	}})
}
