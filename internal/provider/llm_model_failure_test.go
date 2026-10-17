package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func modelFixtureState(t *testing.T, model *LlmModelResource, values string) tfsdk.State {
	t.Helper()
	var schema resource.SchemaResponse
	model.Schema(t.Context(), resource.SchemaRequest{}, &schema)
	raw, err := projectStateJSON([]byte(values), schema.Schema.Type().TerraformType(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	value, err := (tfprotov6.RawState{JSON: raw}).Unmarshal(schema.Schema.Type().TerraformType(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	return tfsdk.State{Schema: schema.Schema, Raw: value}
}

func TestModelDestroyReportsResetFailures(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError, http.StatusNotFound, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPatch {
					t.Errorf("unexpected method %s", r.Method)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if value, ok := body["customPricePerMillionCacheRead"]; !ok || value != nil {
					t.Error("cache overrides were not cleared")
				}
				if body["ignored"] != false {
					t.Error("ignored flag was not reset")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			model := &LlmModelResource{client: c}
			state := modelFixtureState(t, model, `{"id":"12345678-1234-4123-8123-123456789abc","model_id":"llama3","llm_provider":"ollama-native"}`)
			var response resource.DeleteResponse
			model.Delete(t.Context(), resource.DeleteRequest{State: state}, &response)
			wantError := status != http.StatusOK && status != http.StatusNotFound
			if response.Diagnostics.HasError() != wantError {
				t.Fatalf("status %d error=%v", status, response.Diagnostics.HasError())
			}
			if calls != 1 {
				t.Fatalf("expected one reset request, got %d", calls)
			}
		})
	}
}

func TestModelAdoptionRejectsAmbiguousNamesBeforeWriting(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"12345678-1234-4123-8123-123456789abc","modelId":"llama3","provider":"ollama"},{"id":"22345678-1234-4123-8123-123456789abc","modelId":"llama3","provider":"ollama-native"}]`))
	}))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	model := &LlmModelResource{client: c}
	state := modelFixtureState(t, model, `{"model_id":"llama3"}`)
	var created resource.CreateResponse
	model.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &created)
	if !created.Diagnostics.HasError() {
		t.Fatal("ambiguous adoption must fail")
	}
	imported := resource.ImportStateResponse{State: state}
	model.ImportState(t.Context(), resource.ImportStateRequest{ID: "llama3"}, &imported)
	if !imported.Diagnostics.HasError() {
		t.Fatal("ambiguous name import must fail")
	}
	if writes != 0 {
		t.Fatal("ambiguous adoption wrote to an arbitrary model")
	}
}
