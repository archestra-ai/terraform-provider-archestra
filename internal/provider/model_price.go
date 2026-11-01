package provider

import (
	"math/big"

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
	return types.StringPointerValue(wire)
}
