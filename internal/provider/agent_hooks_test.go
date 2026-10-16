package provider

import (
	"context"
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

func TestAccAgentInlineHooks(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-inline-hooks")
	var id, firstHookID string
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
	config := func(hooks string) string {
		return fmt.Sprintf(`resource "archestra_agent" "test" {
 name=%q
 %s
 }`, name, hooks)
	}
	initial := config(`hooks=[{event="session_start",file_name="setup.py",content="print('synthetic')",requirements=[],enabled=false}, {event="pre_tool_use",file_name="setup.py",content="print('synthetic pre tool')",requirements=[],enabled=false}]`)
	updated := config(`hooks=[{event="session_start",file_name="setup.py",content="print('updated synthetic')",requirements=["synthetic-package==0.0.1"],enabled=false}]`)
	read := func() []agentHookWire {
		response, err := api().GetHooksWithResponse(t.Context(), &client.GetHooksParams{AgentId: uuid.MustParse(id)})
		if err != nil || response.JSON200 == nil {
			t.Fatal("unable to read owned hooks", err)
		}
		var out []agentHookWire
		for _, hook := range *response.JSON200 {
			out = append(out, agentHookWire{ID: hook.Id.String(), Event: string(hook.Event), FileName: hook.FileName, Content: hook.Content})
		}
		return out
	}
	same := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_agent.test"].Primary.ID != id {
			return fmt.Errorf("hook edit replaced its agent")
		}
		hooks := read()
		if len(hooks) != 1 || hooks[0].ID != firstHookID || hooks[0].Content != "print('updated synthetic')" {
			return fmt.Errorf("hook update replaced its identity or lost content")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_agent.test", "hooks.#", "2"), func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_agent.test"].Primary.ID
			hooks := read()
			if len(hooks) != 2 {
				return fmt.Errorf("hooks were not created on the agent")
			}
			for _, hook := range hooks {
				if hook.Event == "session_start" {
					firstHookID = hook.ID
				}
			}
			return nil
		})},
		{ResourceName: "archestra_agent.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: same},
		{PreConfig: func() {
			response, err := api().UpdateHookWithBodyWithResponse(t.Context(), uuid.MustParse(firstHookID), "application/json", strings.NewReader(`{"content":"print('out of band')"}`))
			if err != nil || response.JSON200 == nil {
				t.Fatal("unable to modify owned hook", err)
			}
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: same},
		{Config: config(""), Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_agent.test", "hooks.#", "1"))},
		{Config: config(`hooks=[]`), Check: func(*terraform.State) error {
			if len(read()) != 0 {
				return fmt.Errorf("empty hooks did not clear configuration")
			}
			return nil
		}},
		{Config: config(`hooks=[]`), PlanOnly: true},
	}})
}
