package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// Import cannot see configuration, so an imported credential has null
// creation-only inputs in state. ImportState marks the resource in private
// state; only then is a first value bookkeeping rather than a rotation. A
// resource created without a nonce still rotates when one is first set.
const adoptedInputsKey = "adopted_inputs"

const adoptedInputDescription = "Changing this value replaces the resource, except that a first value after import is recorded in place."

func rotationNonceRequiresReplace() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = !req.StateValue.IsNull() || !hasAdoptedInputs(ctx, req.Private, &resp.Diagnostics)
	}, adoptedInputDescription, adoptedInputDescription)
}

func creationInt64RequiresReplace() planmodifier.Int64 {
	return int64planmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.Int64Request, resp *int64planmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = !req.StateValue.IsNull() || !hasAdoptedInputs(ctx, req.Private, &resp.Diagnostics)
	}, adoptedInputDescription, adoptedInputDescription)
}

func markAdoptedInputs(ctx context.Context, resp *resource.ImportStateResponse) {
	if resp.Private == nil {
		return
	}
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, adoptedInputsKey, []byte(`true`))...)
}

func clearAdoptedInputs(ctx context.Context, resp *resource.UpdateResponse) {
	if resp.Private == nil {
		return
	}
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, adoptedInputsKey, nil)...)
}

type privateStateReader interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

func hasAdoptedInputs(ctx context.Context, private privateStateReader, diags *diag.Diagnostics) bool {
	if private == nil {
		return false
	}
	value, d := private.GetKey(ctx, adoptedInputsKey)
	diags.Append(d...)
	return len(value) > 0
}

// recordAdoptedInputs keeps the prior state, records planned values of
// creation-only inputs that need no API call and ends the import adoption.
func recordAdoptedInputs(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse, names ...string) {
	resp.State = req.State
	for _, name := range names {
		var planned attr.Value
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(name), &planned)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), planned)...)
	}
	clearAdoptedInputs(ctx, resp)
}
