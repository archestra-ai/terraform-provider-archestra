package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FlattenStringList converts a backend slice of strings into a TF List value.
//
// Empty backend slices are encoded as a typed *empty list*, never null. This
// is the structural fix for the "perma-diff cluster" — historically the
// provider flattened `[]string{}` to `types.ListNull(StringType)`, which
// disagreed with HCL `attr = []` and caused every refresh to show drift.
//
// A nil slice still maps to a null list (the API didn't return the field at
// all), preserving the absence/empty distinction.
func FlattenStringList(ctx context.Context, api []string, diags *diag.Diagnostics) types.List {
	if api == nil {
		return types.ListNull(types.StringType)
	}
	values := make([]attr.Value, 0, len(api))
	for _, s := range api {
		values = append(values, types.StringValue(s))
	}
	out, d := types.ListValue(types.StringType, values)
	diags.Append(d...)
	return out
}

// FlattenStringSet is the Set equivalent of FlattenStringList. Same nil-vs-empty
// rule.
func FlattenStringSet(ctx context.Context, api []string, diags *diag.Diagnostics) types.Set {
	if api == nil {
		return types.SetNull(types.StringType)
	}
	values := make([]attr.Value, 0, len(api))
	for _, s := range api {
		values = append(values, types.StringValue(s))
	}
	out, d := types.SetValue(types.StringType, values)
	diags.Append(d...)
	return out
}

// FlattenStringMap converts a backend map[string]string into a TF Map value.
// Same nil-vs-empty rule: nil → MapNull, empty map → typed empty MapValue.
func FlattenStringMap(ctx context.Context, api map[string]string, diags *diag.Diagnostics) types.Map {
	if api == nil {
		return types.MapNull(types.StringType)
	}
	values := make(map[string]attr.Value, len(api))
	for k, v := range api {
		values[k] = types.StringValue(v)
	}
	out, d := types.MapValue(types.StringType, values)
	diags.Append(d...)
	return out
}

// PreserveOnNil returns the API-derived value when present, falling back to
// `prior` when the API returned nil. Use only for write-only / computed
// fields where the backend legitimately stops echoing (e.g. an
// id-by-reference that the user supplied once and the backend doesn't
// surface back).
//
// Per Decision A7 (drift-honest reads), this helper is NOT used for
// `Sensitive: true` fields. Sensitive fields are always read from the API
// so UI-side rotations surface in `terraform plan` (`Sensitive: true`
// schema flag handles plan-output masking).
func PreserveOnNil[T any](api *T, prior types.String, encode func(T) types.String) types.String {
	if api == nil {
		return prior
	}
	return encode(*api)
}
