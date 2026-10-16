package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccAgentSkillPolicyResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-skill-policy")
	var c *client.ClientWithResponses
	var skillID, agentID string
	config := func(mode, allowed, excluded string) string {
		return fmt.Sprintf(`
resource "archestra_agent" "test" {
 name=%q
 system_prompt="Synthetic skill-policy lifecycle fixture."
}
resource "archestra_agent_skill_policy" "test" {
 agent_id=archestra_agent.test.id
 mode=%q
 allowed_references=%s
 excluded_references=%s
}
`, name, mode, allowed, excluded)
	}
	reference := func() string { return fmt.Sprintf(`[{source="native",skill_id=%q}]`, skillID) }
	initial := func() string { return config("manual", reference(), "[]") }
	updated := func() string { return config("all", "[]", reference()) }
	setup := func() {
		testAccPreCheck(t)
		testAccRequireAgentRuntimeEnabled(t)
		var err error
		c, err = client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(map[string]any{"content": "---\nname: " + name + "\ndescription: Synthetic policy fixture\n---\nOnly inspect synthetic local fixtures.\n"})
		if err != nil {
			t.Fatal(err)
		}
		created, err := c.CreateSkillWithBodyWithResponse(t.Context(), "application/json", bytes.NewReader(body))
		if err != nil || created.JSON200 == nil {
			t.Fatalf("Unable to create isolated skill fixture: %v", err)
		}
		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(created.Body, &result); err != nil {
			t.Fatal(err)
		}
		skillID = result.ID
		t.Cleanup(func() {
			id, err := uuid.Parse(skillID)
			if err != nil {
				t.Error(err)
				return
			}
			response, err := c.DeleteSkillWithResponse(context.Background(), id)
			if err != nil || response.StatusCode() >= 300 {
				t.Errorf("cleanup skill fixture: %v", err)
			}
		})
	}
	if os.Getenv("TF_ACC") != "" {
		setup()
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial(), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_agent_skill_policy.test", "mode", "manual"), resource.TestCheckResourceAttr("archestra_agent_skill_policy.test", "allowed_references.#", "1"), func(s *terraform.State) error {
			agentID = s.RootModule().Resources["archestra_agent.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_agent_skill_policy.test", ImportState: true, ImportStateVerify: true},
		{Config: initial(), PlanOnly: true},
		{Config: updated(), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_agent_skill_policy.test", "mode", "all"), resource.TestCheckResourceAttr("archestra_agent_skill_policy.test", "allowed_references.#", "0"), resource.TestCheckResourceAttr("archestra_agent_skill_policy.test", "excluded_references.#", "1"))},
		{Config: updated(), PlanOnly: true},
		{Config: updated(), PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
			id := uuid.MustParse(agentID)
			current, err := c.GetAgentActivationSkillPolicyWithResponse(t.Context(), id)
			if err != nil || current.JSON200 == nil {
				t.Fatal("read drift fixture", err)
			}
			body := fmt.Sprintf(`{"expectedRevision":%d,"mode":"manual","operations":[]}`, current.JSON200.Revision)
			response, err := c.PatchAgentActivationSkillPolicyWithBodyWithResponse(t.Context(), id, "application/json", strings.NewReader(body))
			if err != nil || response.JSON200 == nil {
				t.Fatal("write isolated policy drift", err)
			}
		}},
		{Config: updated(), Check: resource.TestCheckResourceAttr("archestra_agent_skill_policy.test", "mode", "all")},
		{Config: fmt.Sprintf(`resource "archestra_agent" "test" {
 name=%q
 system_prompt="Synthetic skill-policy lifecycle fixture."
}`, name), Check: func(_ *terraform.State) error {
			response, err := c.GetAgentActivationSkillPolicyWithResponse(t.Context(), uuid.MustParse(agentID))
			if err != nil || response.JSON200 == nil {
				return fmt.Errorf("policy deletion removed its agent: %w", err)
			}
			var value skillPolicyWire
			if err := json.Unmarshal(response.Body, &value); err != nil {
				return err
			}
			if value.Mode != "manual" || len(value.AllowedReferences) != 0 || len(value.ExcludedReferences) != 0 {
				return fmt.Errorf("policy destroy failed to reset: %+v", value)
			}
			return nil
		}},
	}})
}
