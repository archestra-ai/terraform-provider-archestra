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

func TestAccMCPGatewaySkillPolicies(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-published-skills")
	var gatewayID string
	base := fmt.Sprintf(`
resource "archestra_mcp_gateway" "test" {name=%q}
resource "archestra_skill" "first" {
 name=%q
 description="Synthetic first publication fixture"
 content="Read synthetic local data."
 files={}
 environment_ids=[]
 labels=[]
}
resource "archestra_skill" "second" {
 name=%q
 description="Synthetic second publication fixture"
 content="Inspect synthetic local data."
 files={}
 environment_ids=[]
 labels=[]
}
`, name, name+"-first", name+"-second")
	publication := func(auto bool, ids string) string {
		return fmt.Sprintf(`
resource "archestra_mcp_gateway_skill_publication" "test" {
 gateway_id=archestra_mcp_gateway.test.id
 access_all_skills=%t
 skill_ids=%s
}
`, auto, ids)
	}
	exclusions := func(ids string) string {
		return fmt.Sprintf(`
resource "archestra_mcp_gateway_skill_exclusions" "test" {
 gateway_id=archestra_mcp_gateway.test.id
 excluded_skill_ids=%s
}
`, ids)
	}
	initial := base + publication(false, "[archestra_skill.first.id]") + exclusions("[archestra_skill.second.id]")
	updated := base + publication(true, "[archestra_skill.second.id]") + exclusions("[archestra_skill.first.id]")
	c := func() *client.ClientWithResponses {
		c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_mcp_gateway_skill_publication.test", "access_all_skills", "false"), resource.TestCheckResourceAttr("archestra_mcp_gateway_skill_publication.test", "skill_ids.#", "1"), resource.TestCheckResourceAttr("archestra_mcp_gateway_skill_exclusions.test", "excluded_skill_ids.#", "1"), func(s *terraform.State) error {
			gatewayID = s.RootModule().Resources["archestra_mcp_gateway.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_mcp_gateway_skill_publication.test", ImportState: true, ImportStateVerify: true},
		{ResourceName: "archestra_mcp_gateway_skill_exclusions.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_mcp_gateway_skill_publication.test", "access_all_skills", "true"), resource.TestCheckResourceAttrPair("archestra_mcp_gateway_skill_publication.test", "skill_ids.0", "archestra_skill.second", "id"), resource.TestCheckResourceAttrPair("archestra_mcp_gateway_skill_exclusions.test", "excluded_skill_ids.0", "archestra_skill.first", "id"))},
		{Config: base + publication(false, "[archestra_skill.second.id]") + exclusions("[archestra_skill.first.id]"), Check: resource.TestCheckResourceAttrPair("archestra_mcp_gateway_skill_publication.test", "skill_ids.0", "archestra_skill.second", "id")},
		{Config: base + publication(false, "[]") + exclusions("[]"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_mcp_gateway_skill_publication.test", "skill_ids.#", "0"), resource.TestCheckResourceAttr("archestra_mcp_gateway_skill_exclusions.test", "excluded_skill_ids.#", "0"))},
		{Config: updated},
		{Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
			id := uuid.MustParse(gatewayID)
			publication, err := c().UpdateAgentSkillsWithResponse(t.Context(), id, client.UpdateAgentSkillsJSONRequestBody{AccessAllSkills: false, SkillIds: []uuid.UUID{}})
			if err != nil || publication.JSON200 == nil {
				t.Fatal("Unable to write isolated publication drift", err)
			}
			exclusions, err := c().UpdateAgentSkillExclusionsWithResponse(t.Context(), id, client.UpdateAgentSkillExclusionsJSONRequestBody{ExcludedSkillIds: []uuid.UUID{}})
			if err != nil || exclusions.JSON200 == nil {
				t.Fatal("Unable to write isolated exclusion drift", err)
			}
		}},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_mcp_gateway_skill_publication.test", "access_all_skills", "true"), resource.TestCheckResourceAttr("archestra_mcp_gateway_skill_exclusions.test", "excluded_skill_ids.#", "1"))},
		{Config: updated, PlanOnly: true},
		{Config: base + exclusions("[archestra_skill.first.id]"), Check: func(_ *terraform.State) error {
			id := uuid.MustParse(gatewayID)
			publication, err := c().GetAgentSkillsWithResponse(t.Context(), id)
			if err != nil || publication.JSON200 == nil {
				return fmt.Errorf("publication deletion removed gateway: %w", err)
			}
			if publication.JSON200.AccessAllSkills || len(publication.JSON200.SkillIds) != 0 {
				return fmt.Errorf("publication deletion did not disable and clear")
			}
			exclusions, err := c().GetAgentSkillExclusionsWithResponse(t.Context(), id)
			if err != nil || exclusions.JSON200 == nil || len(exclusions.JSON200.ExcludedSkillIds) != 1 {
				return fmt.Errorf("publication deletion changed independent exclusions: %w", err)
			}
			return nil
		}},
		{Config: base, Check: func(_ *terraform.State) error {
			exclusions, err := c().GetAgentSkillExclusionsWithResponse(t.Context(), uuid.MustParse(gatewayID))
			if err != nil || exclusions.JSON200 == nil || len(exclusions.JSON200.ExcludedSkillIds) != 0 {
				return fmt.Errorf("exclusion deletion did not clear while preserving gateway: %w", err)
			}
			return nil
		}},
	}})
}
