package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestModelSupportedReasoningEfforts(t *testing.T) {
	var ds diag.Diagnostics
	data := LlmModelResourceModel{}
	flattenCurrentModelFields(t.Context(), &data, json.RawMessage(`{"supportedReasoningEfforts":["low","medium","high"]}`), &ds)
	if ds.HasError() || len(data.SupportedReasoningEfforts.Elements()) != 3 {
		t.Fatal("missing reasoning efforts", data.SupportedReasoningEfforts, ds)
	}
	// Older releases omit the field; it must not keep a stale value.
	flattenCurrentModelFields(t.Context(), &data, json.RawMessage(`{}`), &ds)
	if ds.HasError() || !data.SupportedReasoningEfforts.IsNull() {
		t.Fatal("stale reasoning efforts", data.SupportedReasoningEfforts, ds)
	}
}
