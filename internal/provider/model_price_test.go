package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestModelPriceDecimalEquivalenceAndDrift(t *testing.T) {
	for _, tc := range []struct{ previous, wire, want string }{
		{"1.00", "1.00000000", "1.00"},
		{"0.10", "0.10000000", "0.10"},
		{"1e-8", "0.00000001", "1e-8"},
		{"1.00000001", "1.00000002", "1.00000002"},
		{"1.00", "1.25000000", "1.25000000"},
	} {
		t.Run(tc.previous+"/"+tc.wire, func(t *testing.T) {
			got := modelPriceFromAPI(types.StringValue(tc.previous), &tc.wire)
			if got.ValueString() != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
	if !modelPriceFromAPI(types.StringValue("1.00"), nil).IsNull() {
		t.Fatal("cleared price retained an old override")
	}
}

func TestConfiguredModelPriceRefreshPreservesOnlyEquivalentValues(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"`+id+`","modelId":"llama3","provider":"ollama-native","customPricePerMillionInput":"1.00000000","customPricePerMillionOutput":"2.50000001","customPricePerMillionCacheRead":"0.10000000","customPricePerMillionCacheWrite":null}]`)
	}))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	data := LlmModelResourceModel{ID: types.StringValue(id), CustomPricePerMillionInput: types.StringValue("1.00"), CustomPricePerMillionOutput: types.StringValue("2.50"), CustomPricePerMillionCacheRead: types.StringValue("0.10"), CustomPricePerMillionCacheWrite: types.StringValue("0.20")}
	var ds diag.Diagnostics
	(&LlmModelResource{client: api}).readModelState(t.Context(), &data, &ds)
	if ds.HasError() {
		t.Fatal(ds)
	}
	if data.CustomPricePerMillionInput.ValueString() != "1.00" || data.CustomPricePerMillionCacheRead.ValueString() != "0.10" {
		t.Fatal("API padding changed configured values", data)
	}
	if data.CustomPricePerMillionOutput.ValueString() != "2.50000001" || !data.CustomPricePerMillionCacheWrite.IsNull() {
		t.Fatal("real pricing drift was hidden", data)
	}
}
