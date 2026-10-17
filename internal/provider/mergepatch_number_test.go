package provider

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMergePatchKeepsDecimalAndLargeIntegerJSONNumbers(t *testing.T) {
	for _, text := range []string{"0.6", "1.4", "1208925819614629174706176"} {
		t.Run(text, func(t *testing.T) {
			value, _, err := big.ParseFloat(text, 10, 512, big.ToNearestEven)
			if err != nil {
				t.Fatal(err)
			}
			kind := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"value": tftypes.Number}}
			plan := tftypes.NewValue(kind, map[string]tftypes.Value{"value": tftypes.NewValue(tftypes.Number, value)})
			var diagnostics diag.Diagnostics
			patch := MergePatch(t.Context(), plan, tftypes.NewValue(kind, nil), []AttrSpec{{TFName: "value", JSONName: "value", Kind: Scalar}}, &diagnostics)
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			encoded, err := json.Marshal(patch)
			if err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(bytes.NewReader(encoded))
			decoder.UseNumber()
			var wire map[string]any
			if err = decoder.Decode(&wire); err != nil {
				t.Fatal(err)
			}
			number, ok := wire["value"].(json.Number)
			if !ok {
				t.Fatalf("API value encoded as %T instead of a JSON number", wire["value"])
			}
			actual, _, err := big.ParseFloat(number.String(), 10, 512, big.ToNearestEven)
			if err != nil {
				t.Fatal(err)
			}
			if actual.Cmp(value) != 0 {
				t.Fatalf("number lost precision: %s", number)
			}
		})
	}
}
