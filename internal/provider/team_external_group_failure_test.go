package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExternalGroupReadDoesNotDropStateOnFailure(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError, http.StatusNotFound, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status == 200 {
					_, _ = w.Write([]byte(`[]`))
				} else {
					_, _ = w.Write([]byte(`{}`))
				}
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := TeamExternalGroupResource{client: c}
			var schema resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
			raw, err := (tfprotov6.RawState{JSON: []byte(`{"id":"team/mapping","team_id":"team","external_group_id":"fixture"}`)}).Unmarshal(schema.Schema.Type().TerraformType(t.Context()))
			if err != nil {
				t.Fatal(err)
			}
			state := tfsdk.State{Schema: schema.Schema, Raw: raw}
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			wantError := status == 403 || status == 500
			if response.Diagnostics.HasError() != wantError {
				t.Fatalf("status %d unexpected diagnostic", status)
			}
			if response.State.Raw.IsNull() == wantError {
				t.Fatalf("status %d state removal did not match absence", status)
			}
		})
	}
}
