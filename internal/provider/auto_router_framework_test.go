package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const (
	autoRouterTestID   = "22222222-2222-4222-8222-222222222222"
	autoRouterTestKey  = "33333333-3333-4333-8333-333333333333"
	autoRouterTestFast = "44444444-4444-4444-8444-444444444444"
	autoRouterTestNorm = "55555555-5555-4555-8555-555555555555"
	autoRouterTestSmrt = "66666666-6666-4666-8666-666666666666"
)

// fakeAutoRouterAPI mirrors the router API's write semantics at the HTTP
// boundary: request tiers carry modelId/providerApiKeyId while responses
// carry id/apiKey.id, a missing decision model gets the server default, and
// a custom budget sent without a preset clears the preset.
type fakeAutoRouterAPI struct {
	t       *testing.T
	mu      sync.Mutex
	enabled bool
	router  map[string]any
	bodies  []map[string]any
}

func (f *fakeAutoRouterAPI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if req.URL.Path == "/health" {
		fmt.Fprint(w, `{"version":"1.5.0-rc.32"}`)
		return
	}
	if !f.enabled && strings.HasPrefix(req.URL.Path, "/api/auto-routing/") {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"message":"Auto routing is not enabled"}}`)
		return
	}
	item := "/api/auto-routing/routers/" + autoRouterTestID
	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/api/auto-routing/state":
		fmt.Fprint(w, `{}`)
	case req.Method == http.MethodGet && req.URL.Path == "/api/auto-routing/routers":
		list := []any{}
		if f.router != nil {
			list = append(list, f.response())
		}
		_ = json.NewEncoder(w).Encode(list)
	case req.Method == http.MethodPost && req.URL.Path == "/api/auto-routing/routers":
		body := f.decode(req)
		f.router = map[string]any{"name": body["name"], "budgetUsdPerDay": body["budgetUsdPerDay"], "budgetPreset": body["budgetPreset"], "tiers": body["tiers"], "decisionModel": body["decisionModel"]}
		if f.router["decisionModel"] == nil {
			f.router["decisionModel"] = map[string]any{"apiKeyId": nil, "model": "server-default-scorer"}
		}
		_ = json.NewEncoder(w).Encode(f.response())
	case f.router != nil && req.URL.Path == item && req.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(f.response())
	case f.router != nil && req.URL.Path == item && req.Method == http.MethodPut:
		body := f.decode(req)
		for key, value := range body {
			f.router[key] = value
		}
		if _, preset := body["budgetPreset"]; !preset {
			if _, budget := body["budgetUsdPerDay"]; budget {
				f.router["budgetPreset"] = nil
			}
		}
		_ = json.NewEncoder(w).Encode(f.response())
	case f.router != nil && req.URL.Path == item && req.Method == http.MethodDelete:
		f.router = nil
		fmt.Fprint(w, `{"success":true}`)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"message":"Auto router not found"}}`)
	}
}

func (f *fakeAutoRouterAPI) decode(req *http.Request) map[string]any {
	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		f.t.Errorf("invalid request body: %v", err)
	}
	if model, ok := body["decisionModel"].(map[string]any); ok {
		if _, present := model["apiKeyId"]; !present {
			f.t.Error("decisionModel was sent without the apiKeyId the API requires")
		}
	}
	f.bodies = append(f.bodies, body)
	return body
}

func (f *fakeAutoRouterAPI) response() map[string]any {
	tiers := map[string]any{}
	for name, raw := range mustAs[map[string]any](f.router["tiers"]) {
		tier := mustAs[map[string]any](raw)
		tiers[name] = map[string]any{"id": tier["modelId"], "provider": "openai", "modelId": "provider-" + name, "apiKey": map[string]any{"id": tier["providerApiKeyId"], "name": "Key"}}
	}
	return map[string]any{
		"id": autoRouterTestID, "name": f.router["name"], "enabled": true,
		"budgetUsdPerDay": f.router["budgetUsdPerDay"], "budgetPreset": f.router["budgetPreset"],
		"tiers": tiers, "decisionModel": f.router["decisionModel"],
		"usesPastSessions": false, "virtualApiKeyIds": []string{},
		"usage":      map[string]any{"agentCount": 0, "organizationDefault": false},
		"generation": 1, "plannedGeneration": 1, "status": map[string]any{},
	}
}

func autoRouterTestConfig(url, extra string) string {
	return fmt.Sprintf(`provider "archestra" {
  base_url = %q
  api_key  = "fixture-key"
}
resource "archestra_auto_router" "test" {
  name = "auto:engineering"
  tiers = {
    fast   = { model_id = %q, provider_api_key_id = %q }
    normal = { model_id = %q, provider_api_key_id = %q }
    smart  = { model_id = %q, provider_api_key_id = %q }
  }
  %s
}`, url, autoRouterTestFast, autoRouterTestKey, autoRouterTestNorm, autoRouterTestKey, autoRouterTestSmrt, autoRouterTestKey, extra)
}

// Exercises Terraform's real plan/apply/import path against a controlled HTTP
// boundary, not acceptance against a deployed Archestra.
func TestAutoRouterTerraformFramework(t *testing.T) {
	api := &fakeAutoRouterAPI{t: t, enabled: true}
	server := httptest.NewServer(api)
	defer server.Close()

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				// The server default decision model and the absent budget must not drift.
				Config: autoRouterTestConfig(server.URL, `budget_preset = "balanced"`),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("archestra_auto_router.test", "id", autoRouterTestID),
					tfresource.TestCheckResourceAttr("archestra_auto_router.test", "tiers.smart.model_id", autoRouterTestSmrt),
					tfresource.TestCheckResourceAttr("archestra_auto_router.test", "decision_model.model", "server-default-scorer"),
					tfresource.TestCheckNoResourceAttr("archestra_auto_router.test", "decision_model.api_key_id"),
					tfresource.TestCheckNoResourceAttr("archestra_auto_router.test", "budget_usd_per_day"),
				),
			},
			{Config: autoRouterTestConfig(server.URL, `budget_preset = "balanced"`), PlanOnly: true},
			{
				// Adding a custom budget while keeping the preset must not clear the preset.
				Config: autoRouterTestConfig(server.URL, "budget_preset = \"balanced\"\n  budget_usd_per_day = 25"),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("archestra_auto_router.test", "budget_preset", "balanced"),
					tfresource.TestCheckResourceAttr("archestra_auto_router.test", "budget_usd_per_day", "25"),
				),
			},
			{
				// Removing the preset switches to the stored custom budget.
				Config: autoRouterTestConfig(server.URL, `budget_usd_per_day = 25`),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckNoResourceAttr("archestra_auto_router.test", "budget_preset"),
					tfresource.TestCheckResourceAttr("archestra_auto_router.test", "budget_usd_per_day", "25"),
				),
			},
			{
				Config: autoRouterTestConfig(server.URL, "budget_usd_per_day = 25\n  decision_model = { model = \"jev-1.13.0\" }"),
				Check:  tfresource.TestCheckResourceAttr("archestra_auto_router.test", "decision_model.model", "jev-1.13.0"),
			},
			{
				ResourceName:      "archestra_auto_router.test",
				ImportState:       true,
				ImportStateId:     autoRouterTestID,
				ImportStateVerify: true,
			},
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if api.router != nil {
		t.Error("destroy did not delete the router")
	}
	var budgetUpdate map[string]any
	for _, body := range api.bodies {
		if _, ok := body["budgetUsdPerDay"]; ok {
			budgetUpdate = body
			break
		}
	}
	if budgetUpdate == nil || budgetUpdate["budgetPreset"] != "balanced" {
		t.Errorf("budget update did not restate the configured preset: %v", budgetUpdate)
	}
}

func TestAutoRouterTerraformDisabledFeature(t *testing.T) {
	server := httptest.NewServer(&fakeAutoRouterAPI{t: t})
	defer server.Close()
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{{
			Config:      autoRouterTestConfig(server.URL, ""),
			ExpectError: regexp.MustCompile(`ARCHESTRA_AUTO_ROUTING_ENABLED`),
		}},
	})
}

func TestVirtualKeyAutoRouterIDsFromRouters(t *testing.T) {
	const key = "77777777-7777-4777-8777-777777777777"
	routers := []client.AutoRouter{
		{ID: "a", VirtualAPIKeyIDs: []string{"other", key}},
		{ID: "b", VirtualAPIKeyIDs: []string{"other"}},
		{ID: "c", VirtualAPIKeyIDs: []string{key}},
	}
	var diags diag.Diagnostics
	got := virtualKeyAutoRouterIDs(t.Context(), routers, key, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	var ids []string
	diags.Append(got.ElementsAs(t.Context(), &ids, false)...)
	if len(ids) != 2 || ((ids[0] != "a" || ids[1] != "c") && (ids[0] != "c" || ids[1] != "a")) {
		t.Errorf("got %v, want routers a and c", ids)
	}
	empty := virtualKeyAutoRouterIDs(t.Context(), nil, key, &diags)
	if empty.IsNull() || len(empty.Elements()) != 0 {
		t.Errorf("no grants should read as an empty set, got %v", empty)
	}
}
