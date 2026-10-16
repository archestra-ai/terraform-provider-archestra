package provider

import (
	"context"
	"encoding/json"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnvironmentDefaultChangesOnlySelectedKind(t *testing.T) {
	defaults := map[string]*string{"app": nil, "agent": nil, "mcpGateway": nil, "mcpRegistry": nil, "knowledgeSource": nil}
	other := "22222222-2222-4222-8222-222222222222"
	defaults["agent"] = &other
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/api/environments/defaults" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]*string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body) != 1 {
			t.Errorf("default update must own one slot, got %v", body)
		}
		for k, v := range body {
			if k != "app" {
				t.Errorf("changed unrelated kind %s", k)
			}
			defaults[k] = v
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(defaults); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL, client.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	r := &EnvironmentDefaultResource{client: api}
	data := EnvironmentDefaultResourceModel{ResourceType: types.StringValue("app"), EnvironmentID: types.StringValue("11111111-1111-4111-8111-111111111111")}
	var diagnostics diag.Diagnostics
	if !r.set(context.Background(), &data, false, &diagnostics) {
		t.Fatal(diagnostics)
	}
	if data.ID.ValueString() != "app" || data.EnvironmentID.ValueString() != "11111111-1111-4111-8111-111111111111" {
		t.Fatal("did not persist selected default")
	}
	if !r.set(context.Background(), &data, true, &diagnostics) {
		t.Fatal(diagnostics)
	}
	if calls != 2 || defaults["app"] != nil || defaults["agent"] == nil || *defaults["agent"] != other {
		t.Fatal("clearing the managed slot changed another kind")
	}
}
