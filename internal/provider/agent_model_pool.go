package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A selected pool derives both anchors; clearing it restores the direct-model
// configuration, including explicit nulls when no direct model is configured.
func planAgentModelPool(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if resp.Diagnostics.HasError() || req.Plan.Raw.IsNull() {
		return
	}
	var selected, previous types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("model_pool_id"), &selected)...)
	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("model_pool_id"), &previous)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	for _, name := range []string{"model_id", "llm_api_key_id"} {
		var value types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
		if !selected.IsNull() {
			value = types.StringUnknown()
			if !selected.IsUnknown() && selected.Equal(previous) && !req.State.Raw.IsNull() {
				resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(name), &value)...)
			}
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name), value)...)
	}
}
