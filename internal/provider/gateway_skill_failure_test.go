package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestGatewaySkillReadsPreserveStateOnFailureAndRejectInternalAgent(t *testing.T) {
	for _, kind := range []string{"publication", "exclusions"} {
		for _, tc := range []struct {
			name      string
			status    int
			subtype   string
			wantError bool
		}{
			{"forbidden", 403, "mcp_gateway", true}, {"server error", 500, "mcp_gateway", true}, {"gateway deleted", 404, "mcp_gateway", false}, {"internal agent", 200, "agent", true},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				writes := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method != "GET" {
						writes++
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					fmt.Fprintf(w, `{"id":"11111111-1111-4111-8111-111111111111","agentType":%q}`, tc.subtype)
				}))
				defer server.Close()
				c, err := client.NewClientWithResponses(server.URL)
				if err != nil {
					t.Fatal(err)
				}
				var r resource.Resource
				var data any
				id := types.StringValue("11111111-1111-4111-8111-111111111111")
				ids := types.SetValueMust(types.StringType, []attr.Value{})
				if kind == "publication" {
					r = &MCPGatewaySkillPublicationResource{client: c}
					data = &MCPGatewaySkillPublicationResourceModel{ID: id, GatewayID: id, AccessAllSkills: types.BoolValue(false), SkillIDs: ids}
				} else {
					r = &MCPGatewaySkillExclusionsResource{client: c}
					data = &MCPGatewaySkillExclusionsResourceModel{ID: id, GatewayID: id, ExcludedSkillIds: ids}
				}
				var schema resource.SchemaResponse
				r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
				state := tfsdk.State{Schema: schema.Schema}
				if d := state.Set(t.Context(), data); d.HasError() {
					t.Fatal(d)
				}
				response := resource.ReadResponse{State: state}
				r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
				if response.Diagnostics.HasError() != tc.wantError {
					t.Fatal(response.Diagnostics)
				}
				if response.State.Raw.IsNull() != (tc.status == 404) {
					t.Fatal("read removed state without API absence")
				}
				if writes != 0 {
					t.Fatal("read wrote publication policy")
				}
			})
		}
	}
}
