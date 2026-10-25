package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestModelProviderInlineCredentialLifecycle(t *testing.T) {
	const id = "12345678-1234-4123-8123-123456789abc"
	var mu sync.Mutex
	name := "Original provider"
	writes, deletes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.URL.Path, "permission") {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{}`)
			return
		}
		switch req.Method {
		case http.MethodPost, http.MethodPatch:
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			writes++
			if writes == 1 && body["apiKey"] != "synthetic-credential" {
				t.Error("create omitted inline credential")
			}
			if writes == 2 {
				if _, exists := body["apiKey"]; exists {
					t.Error("metadata update resent credential")
				}
			}
			for field := range body {
				if strings.Contains(strings.ToLower(field), "vault") {
					t.Errorf("unexpected wire field %s", field)
				}
			}
			if value, ok := body["name"].(string); ok {
				name = value
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id})
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "name": name, "provider": "ollama", "scope": "personal", "isPrimary": false})
		case http.MethodDelete:
			deletes++
			fmt.Fprint(w, `{}`)
		default:
			t.Errorf("unexpected method %s", req.Method)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := &LLMProviderApiKeyResource{client: api}
	var schema resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
	kind := schema.Schema.Type().TerraformType(t.Context())
	encoded, err := projectStateJSON([]byte(`{"name":"Original provider","llm_provider":"ollama","api_key":"synthetic-credential","is_primary":false}`), kind)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := (tfprotov6.RawState{JSON: encoded}).Unmarshal(kind)
	if err != nil {
		t.Fatal(err)
	}
	var created resource.CreateResponse
	created.State.Schema = schema.Schema
	r.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: schema.Schema, Raw: raw}}, &created)
	if created.Diagnostics.HasError() {
		t.Fatal(created.Diagnostics)
	}
	var before LLMProviderApiKeyResourceModel
	if d := created.State.Get(t.Context(), &before); d.HasError() {
		t.Fatal(d)
	}
	if before.ID.ValueString() != id || before.ApiKey.ValueString() != "synthetic-credential" {
		t.Fatal("create/refresh lost binding or credential")
	}
	desired := before
	desired.Name = types.StringValue("Renamed provider")
	plan := tfsdk.Plan{Schema: schema.Schema}
	if d := plan.Set(t.Context(), &desired); d.HasError() {
		t.Fatal(d)
	}
	updated := resource.UpdateResponse{State: created.State}
	r.Update(t.Context(), resource.UpdateRequest{State: created.State, Plan: plan}, &updated)
	if updated.Diagnostics.HasError() {
		t.Fatal(updated.Diagnostics)
	}
	var after LLMProviderApiKeyResourceModel
	if d := updated.State.Get(t.Context(), &after); d.HasError() {
		t.Fatal(d)
	}
	if after.ApiKey.ValueString() != before.ApiKey.ValueString() || after.ID.ValueString() != id || after.Name.ValueString() != "Renamed provider" {
		t.Fatal("update lost retained state")
	}
	var deleted resource.DeleteResponse
	r.Delete(t.Context(), resource.DeleteRequest{State: updated.State}, &deleted)
	if deleted.Diagnostics.HasError() {
		t.Fatal(deleted.Diagnostics)
	}
	if writes != 2 || deletes != 1 {
		t.Fatal("unexpected API writes", writes, deletes)
	}
}

func TestTeamBindingReadOnlyUsesExternalGroups(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		if req.Method != "GET" || req.URL.Path != "/api/teams/team-a/external-groups" {
			t.Errorf("unexpected team API call %s %s", req.Method, req.URL.Path)
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"mapping-a","teamId":"team-a","groupIdentifier":"engineering"}]`)
	}))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	data := TeamResourceModel{ID: types.StringValue("team-a"), ExternalGroupIDs: types.SetNull(types.StringType)}
	var diagnostics diag.Diagnostics
	(&TeamResource{client: api}).readBindings(t.Context(), &data, &diagnostics)
	if diagnostics.HasError() || calls != 1 {
		t.Fatal("supported team read failed", diagnostics, calls)
	}
	var groups []string
	if d := data.ExternalGroupIDs.ElementsAs(t.Context(), &groups, false); d.HasError() || len(groups) != 1 || groups[0] != "engineering" {
		t.Fatal("lost group mapping", d)
	}
}
