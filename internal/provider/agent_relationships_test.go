package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccAgentInlineRelationships(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-inline-agent")
	var once sync.Once
	var id string
	api := func() *client.ClientWithResponses {
		c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	base := fmt.Sprintf(`
 variable "inline_connector_id" {type=string}
 resource "archestra_agent" "first" {name=%q}
 resource "archestra_agent" "second" {name=%q}
 resource "archestra_skill" "fixture" {
 name=%q
 description="Synthetic inline agent policy"
 content="Inspect only synthetic fixtures."
 files={}
 labels=[]
 environment_ids=[]
 }
 `, name+"-first", name+"-second", name+"-skill")
	base += fmt.Sprintf(`resource "archestra_remote_agent" "first" { source=%q }
`, testRemoteCard(name+"-remote"))
	config := func(fields string) string {
		return base + fmt.Sprintf(`
 resource "archestra_agent" "test" {
 name=%q
 system_prompt="Synthetic inline relationship fixture."
 %s
 }
 `, name, fields)
	}
	initial := config(`subagent_ids=[archestra_agent.first.id]
 remote_agent_ids=[]
 excluded_subagent_ids=[archestra_agent.second.id]
 excluded_connector_ids=[var.inline_connector_id]
 skills={mode="manual",allowed=[{source="native",skill_id=archestra_skill.fixture.id}],excluded=[]}`)
	updated := config(`subagent_ids=[archestra_agent.second.id]
 remote_agent_ids=[archestra_remote_agent.first.id]
 excluded_subagent_ids=[archestra_agent.first.id]
 excluded_connector_ids=[]
 skills={mode="all",allowed=[],excluded=[{source="native",skill_id=archestra_skill.fixture.id}]}`)
	cleared := config(`subagent_ids=[]
 remote_agent_ids=[]
 excluded_subagent_ids=[]
 excluded_connector_ids=[]
 skills={mode="manual",allowed=[],excluded=[]}`)
	drift := func() {
		agentID := uuid.MustParse(id)
		r, err := api().SyncAgentDelegationsWithResponse(t.Context(), agentID, client.SyncAgentDelegationsJSONRequestBody{TargetAgentIds: []uuid.UUID{}})
		if err != nil || r.JSON200 == nil {
			t.Fatal("unable to modify owned subagent assignments", err)
		}
		r2, err := api().UpdateAgentSubagentExclusionsWithResponse(t.Context(), agentID, client.UpdateAgentSubagentExclusionsJSONRequestBody{ExcludedSubagentIds: []uuid.UUID{}})
		if err != nil || r2.JSON200 == nil {
			t.Fatal("unable to modify owned exclusions", err)
		}
		policy, err := api().GetAgentActivationSkillPolicyWithResponse(t.Context(), agentID)
		if err != nil || policy.JSON200 == nil {
			t.Fatal("unable to read owned skill policy", err)
		}
		body := fmt.Sprintf(`{"expectedRevision":%d,"mode":"manual","operations":[]}`, policy.JSON200.Revision)
		r3, err := api().PatchAgentActivationSkillPolicyWithBodyWithResponse(t.Context(), agentID, "application/json", strings.NewReader(body))
		if err != nil || r3.JSON200 == nil {
			t.Fatal("unable to modify owned policy", err)
		}
	}
	unchanged := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_agent.test"].Primary.ID != id {
			return fmt.Errorf("relationship edit replaced the agent")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		testAccPreCheck(t)
		testAccRequireAgentRuntimeEnabled(t)
		once.Do(func() { t.Setenv("TF_VAR_inline_connector_id", testKnowledgeConnectorFixture(t, name+"-connector")) })
	}, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_agent.test", "subagent_ids.#", "1"), resource.TestCheckResourceAttr("archestra_agent.test", "excluded_subagent_ids.#", "1"), resource.TestCheckResourceAttr("archestra_agent.test", "excluded_connector_ids.#", "1"), resource.TestCheckResourceAttr("archestra_agent.test", "skills.mode", "manual"), func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_agent.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_agent.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeTestCheckFunc(unchanged, resource.TestCheckResourceAttr("archestra_agent.test", "skills.mode", "all"), resource.TestCheckResourceAttr("archestra_agent.test", "remote_agent_ids.#", "1"), resource.TestCheckResourceAttr("archestra_agent.test", "excluded_connector_ids.#", "0"))},
		{PreConfig: drift, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: resource.ComposeTestCheckFunc(unchanged, resource.TestCheckResourceAttr("archestra_agent.test", "subagent_ids.#", "1"), resource.TestCheckResourceAttr("archestra_agent.test", "skills.mode", "all"))},
		{Config: config(""), Check: resource.ComposeTestCheckFunc(unchanged, resource.TestCheckResourceAttr("archestra_agent.test", "subagent_ids.#", "1"), resource.TestCheckResourceAttr("archestra_agent.test", "skills.mode", "all"))},
		{Config: cleared, Check: resource.ComposeTestCheckFunc(unchanged, resource.TestCheckResourceAttr("archestra_agent.test", "subagent_ids.#", "0"), resource.TestCheckResourceAttr("archestra_agent.test", "excluded_subagent_ids.#", "0"), resource.TestCheckResourceAttr("archestra_agent.test", "skills.allowed.#", "0"), resource.TestCheckResourceAttr("archestra_agent.test", "skills.excluded.#", "0"))},
		{Config: cleared, PlanOnly: true},
	}})
}
