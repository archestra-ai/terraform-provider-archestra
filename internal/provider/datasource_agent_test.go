package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAgentLookupNameRequiresUniqueExactMatchAcrossPages(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if r.Method != "GET" || r.URL.Query().Get("agentType") != "agent" || r.URL.Query().Get("name") != "Support" {
					t.Error("lookup attempted an unrelated request")
				}
				if r.URL.Query().Get("offset") == "0" {
					name := "Other"
					if count > 0 {
						name = "Support"
					}
					fmt.Fprintf(w, `{"data":[{"id":"11111111-1111-4111-8111-111111111111","name":%q,"agentType":"agent"}],"pagination":{"hasNext":true}}`, name)
				} else {
					name := "Other"
					if count > 1 {
						name = "Support"
					}
					fmt.Fprintf(w, `{"data":[{"id":"22222222-2222-4222-8222-222222222222","name":%q,"agentType":"agent"}],"pagination":{"hasNext":false}}`, name)
				}
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			id, err := (&AgentDataSource{client: c}).agentIDByName(t.Context(), "Support")
			if (err == nil) != (count == 1) || calls != 2 {
				t.Fatal("name lookup did not inspect all exact matches", id, err, calls)
			}
		})
	}
}

func TestAccAgentDataSourceRelationships(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-agent-lookup")
	config := func(prompt string) string {
		return fmt.Sprintf(`
resource "archestra_agent" "child" {name=%q}
resource "archestra_agent" "test" {
 name=%q
 system_prompt=%q
 subagent_ids=[archestra_agent.child.id]
 skills={mode="manual",allowed=[],excluded=[]}
 hooks=[{event="session_start",file_name="lookup.sh",content="echo lookup",requirements=[],enabled=false}]
}
data "archestra_agent" "by_id" {id=archestra_agent.test.id}
data "archestra_agent" "by_name" {
 name=archestra_agent.test.name
 depends_on=[archestra_agent.test]
}
`, name+"-child", name, prompt)
	}
	checks := resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttrPair("data.archestra_agent.by_id", "id", "archestra_agent.test", "id"),
		resource.TestCheckResourceAttrPair("data.archestra_agent.by_name", "id", "archestra_agent.test", "id"),
		resource.TestCheckResourceAttrPair("data.archestra_agent.by_name", "system_prompt", "archestra_agent.test", "system_prompt"),
		resource.TestCheckResourceAttr("data.archestra_agent.by_id", "subagent_ids.#", "1"),
		resource.TestCheckResourceAttr("data.archestra_agent.by_name", "subagent_ids.#", "1"),
		resource.TestCheckResourceAttr("data.archestra_agent.by_name", "skills.mode", "manual"),
		resource.TestCheckResourceAttr("data.archestra_agent.by_name", "hooks.#", "1"),
	)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("Initial synthetic lookup fixture."), Check: checks},
		{Config: config("Updated synthetic lookup fixture."), Check: checks},
		{Config: config("Updated synthetic lookup fixture."), PlanOnly: true},
	}})
}
