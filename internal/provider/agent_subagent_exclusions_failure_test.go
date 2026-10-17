package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSubagentExclusionsReadFailuresPreserveState(t *testing.T) {
	for _, status := range []int{403, 500, 404, 200} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, `{"excludedSubagentIds":[]}`)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := AgentSubagentExclusionsResource{client: c}
			var s resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &s)
			state := tfsdk.State{Schema: s.Schema}
			ids, d := types.SetValueFrom(t.Context(), types.StringType, []string{})
			if d.HasError() {
				t.Fatal(d)
			}
			value := AgentSubagentExclusionsResourceModel{ID: types.StringValue("11111111-1111-4111-8111-111111111111"), AgentID: types.StringValue("11111111-1111-4111-8111-111111111111"), ExcludedSubagentIds: ids}
			if d := state.Set(t.Context(), &value); d.HasError() {
				t.Fatal(d)
			}
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			if response.Diagnostics.HasError() != (status == 403 || status == 500) {
				t.Fatal(response.Diagnostics)
			}
			if response.State.Raw.IsNull() != (status == 404) {
				t.Fatal("State removal requires API absence")
			}
		})
	}
}
func TestSubagentExclusionsRejectFilteredOrSelfTargets(t *testing.T) {
	for _, self := range []bool{false, true} {
		t.Run(fmt.Sprint(self), func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writes++
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"excludedSubagentIds":[]}`)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := AgentSubagentExclusionsResource{client: c}
			id := "22222222-2222-4222-8222-222222222222"
			if self {
				id = "11111111-1111-4111-8111-111111111111"
			}
			ids, d := types.SetValueFrom(t.Context(), types.StringType, []string{id})
			if d.HasError() {
				t.Fatal(d)
			}
			plan := AgentSubagentExclusionsResourceModel{AgentID: types.StringValue("11111111-1111-4111-8111-111111111111"), ExcludedSubagentIds: ids}
			var diags diag.Diagnostics
			if r.replaceExclusions(t.Context(), &plan, &diags) || !diags.HasError() {
				t.Fatal("Filtered request falsely reported successful state")
			}
			if self && writes != 0 {
				t.Fatal("Self-reference should be refused before a policy write")
			}
		})
	}
}
