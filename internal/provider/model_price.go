package provider

import (
	"math/big"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Keep configured formatting only when the API reports exactly the same price.
func modelPriceFromAPI(previous types.String, wire *string) types.String {
	if wire != nil && !previous.IsNull() && !previous.IsUnknown() {
		old, oldOK := new(big.Rat).SetString(previous.ValueString())
		current, currentOK := new(big.Rat).SetString(*wire)
		if oldOK && currentOK && old.Cmp(current) == 0 {
			return previous
		}
	}
	// Fresh imports retain the historical two-decimal spelling without rounding.
	if wire != nil && (previous.IsNull() || previous.IsUnknown()) {
		if _, valid := new(big.Rat).SetString(*wire); valid && !strings.ContainsAny(*wire, "eE/") {
			price := *wire
			if dot := strings.IndexByte(price, '.'); dot < 0 {
				price += ".00"
			} else {
				price = strings.TrimRight(price, "0")
				for len(price)-dot-1 < 2 {
					price += "0"
				}
			}
			return types.StringValue(price)
		}
	}
	return types.StringPointerValue(wire)
}
