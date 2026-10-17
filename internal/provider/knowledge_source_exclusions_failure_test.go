package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKnowledgeSourceExclusionsReadFailurePreservesState(t *testing.T) {
	for _, status := range []int{403, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, `{}`)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := AgentKnowledgeSourceExclusionsResource{client: c}
			var schema resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
			value := AgentKnowledgeSourceExclusionsResourceModel{ID: types.StringValue("11111111-1111-4111-8111-111111111111"), AgentID: types.StringValue("11111111-1111-4111-8111-111111111111"), ExcludedConnectorIds: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("22222222-2222-4222-8222-222222222222")})}
			state := tfsdk.State{Schema: schema.Schema}
			if d := state.Set(t.Context(), &value); d.HasError() {
				t.Fatal(d)
			}
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			if !response.Diagnostics.HasError() || !response.State.Raw.Equal(state.Raw) {
				t.Fatal("failed read must preserve the prior exclusion set", response.Diagnostics)
			}
		})
	}
}

func TestKnowledgeSourceExclusionsRefusesSilentlyDroppedConnectors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPut {
			t.Error("unexpected mutation", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"excludedConnectorIds":[]}`)
	}))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := AgentKnowledgeSourceExclusionsResource{client: c}
	value := AgentKnowledgeSourceExclusionsResourceModel{AgentID: types.StringValue("11111111-1111-4111-8111-111111111111"), ExcludedConnectorIds: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("22222222-2222-4222-8222-222222222222")})}
	var diagnostics diag.Diagnostics
	if r.replaceExclusions(t.Context(), &value, &diagnostics) || !diagnostics.HasError() {
		t.Fatal("silently dropped connector must not be recorded as successful state")
	}
	if calls != 1 {
		t.Fatal("invalid mutation was retried", calls)
	}
}
