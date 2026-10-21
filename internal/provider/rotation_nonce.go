package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// Import cannot see configuration, so an adopted credential has null
// creation-only inputs in state. A first value is bookkeeping; only a change
// of an existing value replaces the resource.
const adoptedInputDescription = "Changing an existing value replaces the resource; a first value after import is recorded in place."

func rotationNonceRequiresReplace() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = !req.StateValue.IsNull()
	}, adoptedInputDescription, adoptedInputDescription)
}

func creationInt64RequiresReplace() planmodifier.Int64 {
	return int64planmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.Int64Request, resp *int64planmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = !req.StateValue.IsNull()
	}, adoptedInputDescription, adoptedInputDescription)
}

// recordAdoptedInputs keeps the prior state and records planned values of
// creation-only inputs that need no API call.
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
}
