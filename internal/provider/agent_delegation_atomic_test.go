package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDelegationCreateUsesOnlyAtomicEdgeMutation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{"created", 200, `{"created":true}`, ""},
		{"already-owned", 200, `{"created":false}`, "Import it instead"},
		{"unsupported", 404, `{}`, "Expected 200 OK"},
		{"denied", 403, `{}`, "Expected 200 OK"},
		{"conflict", 409, `{}`, "Expected 200 OK"},
		{"malformed", 200, `{`, "Unable to create"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			agentID := "11111111-1111-4111-8111-111111111111"
			targetID := "22222222-2222-4222-8222-222222222222"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != http.MethodPost || req.URL.Path != "/api/agents/"+agentID+"/delegations/"+targetID {
					t.Errorf("unexpected request could affect another edge: %s %s", req.Method, req.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := AgentDelegationResource{client: c}
			var schema resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
			value := AgentDelegationResourceModel{ID: types.StringUnknown(), AgentID: types.StringValue(agentID), TargetAgentID: types.StringValue(targetID)}
			plan := tfsdk.Plan{Schema: schema.Schema}
			if d := plan.Set(t.Context(), &value); d.HasError() {
				t.Fatal(d)
			}
			response := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
			r.Create(t.Context(), resource.CreateRequest{Plan: plan}, &response)
			if calls != 1 {
				t.Fatalf("mutation was retried or fell back to a full-set operation: %d requests", calls)
			}
			if tc.wantError != "" {
				if !response.Diagnostics.HasError() || !strings.Contains(fmt.Sprint(response.Diagnostics), tc.wantError) {
					t.Fatalf("missing expected diagnostic %q: %v", tc.wantError, response.Diagnostics)
				}
				return
			}
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
			var actual AgentDelegationResourceModel
			if d := response.State.Get(t.Context(), &actual); d.HasError() {
				t.Fatal(d)
			}
			if actual.ID.ValueString() != agentID+":"+targetID {
				t.Fatal("edge identity was not retained", actual.ID)
			}
		})
	}
}
