package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"time"
)

// Canonicalize raw JSON dates and dates marshalled from generated client structs identically.
func canonicalAPITimestamp(value *time.Time) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(value.UTC().Format(time.RFC3339Nano))
}
