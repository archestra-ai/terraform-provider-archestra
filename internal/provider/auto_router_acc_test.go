package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccAutoRouterFixtures returns an organization-wide provider key and a
// tool-capable chat model it serves, or skips when the deployment cannot host
// an auto router (older API, auto routing disabled, or no such model synced).
func testAccAutoRouterFixtures(t *testing.T) (keyID, modelID string) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		return "", ""
	}
	if !testAccAPIVersionAtLeast(t, "1.5.0-rc.29") {
		t.Skip("Auto routers require API 1.5.0-rc.29")
	}
	testAccPreCheck(t)
	api, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	routers, err := client.NewAutoRouterClient(api)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := routers.Enabled(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Skip("Auto routing is disabled; set ARCHESTRA_AUTO_ROUTING_ENABLED=true on the API")
	}
	parsed, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
	if err != nil || parsed.Scheme != "http" || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Skip("Auto router fixtures create provider keys and require an isolated loopback API")
	}
	transport := http.Client{Timeout: 45 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(ctx context.Context, method, endpoint, body string, result any) int {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		// #nosec G704 -- The API URL is validated as loopback HTTP and redirects are refused.
		req, err := http.NewRequestWithContext(ctx, method, parsed.String()+endpoint, reader)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		// #nosec G704 -- The API URL is validated as loopback HTTP and redirects are refused.
		resp, err := transport.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if result != nil && resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode
	}
	var key struct {
		ID string `json:"id"`
	}
	payload, _ := json.Marshal(map[string]any{"name": acctest.RandomWithPrefix("tf-auto-router"), "provider": "ollama", "baseUrl": os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL"), "apiKey": "synthetic-router-key", "isPrimary": false, "shared": true, "initialGrants": []map[string]any{{"subject": map[string]any{"type": "organization", "id": "*"}, "actions": []string{"read", "use"}}}})
	if status := call(t.Context(), http.MethodPost, "/api/llm-provider-api-keys", string(payload), &key); status != http.StatusOK || key.ID == "" {
		t.Fatalf("creating the fixture provider key returned HTTP %d", status)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		call(ctx, http.MethodDelete, "/api/llm-provider-api-keys/"+key.ID, "", nil)
	})
	var models []struct {
		DBID         *string `json:"dbId"`
		Capabilities *struct {
			SupportsToolCalling *bool    `json:"supportsToolCalling"`
			OutputModalities    []string `json:"outputModalities"`
		} `json:"capabilities"`
	}
	call(t.Context(), http.MethodGet, "/api/llm-models/available?apiKeyId="+url.QueryEscape(key.ID), "", &models)
	for _, model := range models {
		if model.DBID == nil || model.Capabilities == nil || (model.Capabilities.SupportsToolCalling != nil && !*model.Capabilities.SupportsToolCalling) {
			continue
		}
		textOutput := model.Capabilities.OutputModalities == nil
		for _, modality := range model.Capabilities.OutputModalities {
			textOutput = textOutput || modality == "text"
		}
		if textOutput {
			return key.ID, *model.DBID
		}
	}
	t.Skip("The fixture provider key serves no tool-capable chat model")
	return "", ""
}

func TestAccAutoRouterLifecycle(t *testing.T) {
	keyID, modelID := testAccAutoRouterFixtures(t)
	name := "auto:" + acctest.RandString(10)
	config := func(budget, references string) string {
		return fmt.Sprintf(`resource "archestra_auto_router" "test" {
  name = %q
  %s
  tiers = {
    fast   = { model_id = %q, provider_api_key_id = %q }
    normal = { model_id = %q, provider_api_key_id = %q }
    smart  = { model_id = %q, provider_api_key_id = %q }
  }
}
data "archestra_auto_router" "test" {
  name       = archestra_auto_router.test.name
  depends_on = [archestra_auto_router.test]
}
%s`, name, budget, modelID, keyID, modelID, keyID, modelID, keyID, references)
	}
	// A standard virtual key needs a provider key or a model pool; a router
	// grant alone is not enough, so the key also maps the fixture key.
	references := fmt.Sprintf(`resource "archestra_agent" "routed" {
  name           = "tf-auto-router-agent"
  auto_router_id = archestra_auto_router.test.id
}
resource "archestra_virtual_key" "routed" {
  name            = "tf-auto-router-key"
  provider_api_keys = [{
    provider            = "ollama"
    provider_api_key_id = %q
  }]
  auto_router_ids = [archestra_auto_router.test.id]
}`, keyID)
	initial := config(`budget_preset = "balanced"`, references)
	updated := config("budget_usd_per_day = 12.5", references)
	cleared := config("budget_usd_per_day = 12.5", fmt.Sprintf(`resource "archestra_agent" "routed" {
  name = "tf-auto-router-agent"
}
resource "archestra_virtual_key" "routed" {
  name            = "tf-auto-router-key"
  provider_api_keys = [{
    provider            = "ollama"
    provider_api_key_id = %q
  }]
  auto_router_ids = []
}`, keyID))
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("archestra_auto_router.test", "budget_preset", "balanced"),
			resource.TestCheckResourceAttrSet("archestra_auto_router.test", "decision_model.model"),
			resource.TestCheckResourceAttrPair("data.archestra_auto_router.test", "id", "archestra_auto_router.test", "id"),
			resource.TestCheckResourceAttrPair("archestra_agent.routed", "auto_router_id", "archestra_auto_router.test", "id"),
			resource.TestCheckResourceAttr("archestra_virtual_key.routed", "auto_router_ids.#", "1"),
		)},
		// virtual_api_key_ids comes from the virtual key's own grant, written
		// after the router was created, so state catches up on the next refresh.
		{RefreshState: true, Check: resource.TestCheckResourceAttr("archestra_auto_router.test", "virtual_api_key_ids.#", "1")},
		{ResourceName: "archestra_auto_router.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckNoResourceAttr("archestra_auto_router.test", "budget_preset"),
			resource.TestCheckResourceAttr("archestra_auto_router.test", "budget_usd_per_day", "12.5"),
		)},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckNoResourceAttr("archestra_agent.routed", "auto_router_id"),
			resource.TestCheckResourceAttr("archestra_virtual_key.routed", "auto_router_ids.#", "0"),
		)},
		{Config: cleared, PlanOnly: true},
	}})
}
