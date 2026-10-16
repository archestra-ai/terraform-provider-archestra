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

func TestAccGatewayInlineRelationships(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-inline-gateway")
	var id string
	base := fmt.Sprintf(`
 resource "archestra_agent" "first" {name=%q}
 resource "archestra_agent" "second" {name=%q}
 resource "archestra_remote_agent" "remote" {source=%q}
 resource "archestra_skill" "first" {
 name=%q
 description="Synthetic gateway publication"
 content="Inspect only synthetic fixtures."
 files={}
 labels=[]
 environment_ids=[]
 }
 resource "archestra_skill" "second" {
 name=%q
 description="Synthetic alternate publication"
 content="Inspect only synthetic fixtures."
 files={}
 labels=[]
 environment_ids=[]
 }
 `, name+"-first", name+"-second", testRemoteCard(name+"-remote"), name+"-skill-first", name+"-skill-second")
	config := func(fields string) string {
		return base + fmt.Sprintf(`
 resource "archestra_mcp_gateway" "test" {
 name=%q
 %s
 }`, name, fields)
	}
	initial := config(`subagent_ids=[archestra_agent.first.id]
 remote_agent_ids=[archestra_remote_agent.remote.id]
 excluded_subagent_ids=[archestra_agent.second.id]
 excluded_connector_ids=[]
 tools=[]
 excluded_tool_ids=[]
 skills={mode="manual",ids=[archestra_skill.first.id],excluded_ids=[]}`)
	updated := config(`subagent_ids=[archestra_agent.second.id]
 remote_agent_ids=[]
 excluded_subagent_ids=[archestra_agent.first.id]
 excluded_connector_ids=[]
 tools=[]
 excluded_tool_ids=[]
 skills={mode="all",ids=[archestra_skill.second.id],excluded_ids=[archestra_skill.first.id]}`)
	cleared := config(`subagent_ids=[]
 remote_agent_ids=[]
 excluded_subagent_ids=[]
 excluded_connector_ids=[]
 tools=[]
 excluded_tool_ids=[]
 skills={mode="manual",ids=[],excluded_ids=[]}`)
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
	same := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_mcp_gateway.test"].Primary.ID != id {
			return fmt.Errorf("relationship edit replaced the gateway")
		}
		return nil
	}
	checks := func(mode, count string) resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "skills.mode", mode), resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "subagent_ids.#", count))
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "subagent_ids.#", "1"), resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "remote_agent_ids.#", "1"), resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "skills.ids.#", "1"), func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_mcp_gateway.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_mcp_gateway.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeTestCheckFunc(checks("all", "1"), resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "remote_agent_ids.#", "0"), resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "skills.excluded_ids.#", "1"))},
		{PreConfig: func() {
			r, err := api().SyncAgentDelegationsWithResponse(t.Context(), uuid.MustParse(id), client.SyncAgentDelegationsJSONRequestBody{TargetAgentIds: []uuid.UUID{}})
			if err != nil || r.JSON200 == nil {
				t.Fatal("unable to modify owned gateway subagents", err)
			}
			r2, err := api().UpdateAgentSkillsWithResponse(t.Context(), uuid.MustParse(id), client.UpdateAgentSkillsJSONRequestBody{AccessAllSkills: false, SkillIds: []uuid.UUID{}})
			if err != nil || r2.JSON200 == nil {
				t.Fatal("unable to modify owned gateway skills", err)
			}
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: checks("all", "1")},
		{Config: config(""), Check: resource.ComposeTestCheckFunc(checks("all", "1"), resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "skills.excluded_ids.#", "1"))},
		{Config: cleared, Check: resource.ComposeTestCheckFunc(checks("manual", "0"), resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "skills.ids.#", "0"), resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "skills.excluded_ids.#", "0"))},
		{Config: cleared, PlanOnly: true},
	}})
}
