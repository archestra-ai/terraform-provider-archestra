package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// The fixtures never contact an external service or create credentials.
func testKnowledgeConnectorFixture(t *testing.T, name string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"name": name, "connectorType": "web_crawler", "enabled": false, "config": map[string]any{"type": "web_crawler", "startUrl": "http://127.0.0.1:19999/", "allowPrivateNetwork": true, "maxPages": 1}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, os.Getenv("ARCHESTRA_BASE_URL")+"/api/connectors", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
	request.Header.Set("Content-Type", "application/json")
	c := &http.Client{Timeout: 30 * time.Second}
	response, err := c.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var connector struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err = json.NewDecoder(response.Body).Decode(&connector); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || connector.ID == "" || connector.Enabled {
		t.Fatalf("disabled connector fixture failed: HTTP %d", response.StatusCode)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodDelete, os.Getenv("ARCHESTRA_BASE_URL")+"/api/connectors/"+connector.ID, nil)
		if err != nil {
			t.Error(err)
			return
		}
		request.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		result, err := c.Do(request)
		if err != nil {
			t.Error(err)
			return
		}
		defer result.Body.Close()
		_, _ = io.Copy(io.Discard, result.Body)
		if result.StatusCode != 200 && result.StatusCode != 204 && result.StatusCode != 404 {
			t.Errorf("owned connector cleanup: HTTP %d", result.StatusCode)
		}
	})
	return connector.ID
}

func TestAccAgentKnowledgeSourceExclusionsResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-knowledge-exclusions")
	var setup sync.Once
	var agentID string
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
	base := fmt.Sprintf(`
variable "first_connector_id" { type = string }
variable "second_connector_id" { type = string }
resource "archestra_agent" "parent" {
 name=%q
 system_prompt="Synthetic knowledge exclusion fixture."
 access_all_tools=true
}
`, name)
	config := func(ids string) string {
		return base + fmt.Sprintf(`
resource "archestra_agent_knowledge_source_exclusions" "test" {
 agent_id=archestra_agent.parent.id
 excluded_connector_ids=%s
}
`, ids)
	}
	initial, updated := config("[var.first_connector_id]"), config("[var.second_connector_id]")
	resource.Test(t, resource.TestCase{PreCheck: func() {
		testAccPreCheck(t)
		setup.Do(func() {
			t.Setenv("TF_VAR_first_connector_id", testKnowledgeConnectorFixture(t, name+"-first"))
			t.Setenv("TF_VAR_second_connector_id", testKnowledgeConnectorFixture(t, name+"-second"))
		})
	}, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_agent_knowledge_source_exclusions.test", "excluded_connector_ids.#", "1"), func(s *terraform.State) error {
			agentID = s.RootModule().Resources["archestra_agent.parent"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_agent_knowledge_source_exclusions.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: func(s *terraform.State) error {
			if s.RootModule().Resources["archestra_agent_knowledge_source_exclusions.test"].Primary.Attributes["excluded_connector_ids.0"] != os.Getenv("TF_VAR_second_connector_id") {
				return fmt.Errorf("replacement connector was not persisted")
			}
			return nil
		}},
		{Config: updated, PlanOnly: true},
		{Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
			response, err := apiClient().UpdateAgentKnowledgeSourceExclusionsWithResponse(t.Context(), uuid.MustParse(agentID), client.UpdateAgentKnowledgeSourceExclusionsJSONRequestBody{ExcludedConnectorIds: []uuid.UUID{}})
			if err != nil || response.JSON200 == nil {
				t.Fatal("Unable to inject owned exclusion drift", err)
			}
		}},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_agent_knowledge_source_exclusions.test", "excluded_connector_ids.#", "1")},
		{Config: config("[]"), Check: resource.TestCheckResourceAttr("archestra_agent_knowledge_source_exclusions.test", "excluded_connector_ids.#", "0")},
		{Config: updated},
		{Config: base, Check: func(_ *terraform.State) error {
			response, err := apiClient().GetAgentKnowledgeSourceExclusionsWithResponse(t.Context(), uuid.MustParse(agentID))
			if err != nil {
				return err
			}
			if response.JSON200 == nil || len(response.JSON200.ExcludedConnectorIds) != 0 {
				return fmt.Errorf("delete failed to clear exclusions while preserving the parent")
			}
			return nil
		}},
	}})
}
