package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Nested computed relationships can mark unrelated metadata unknown before
// attribute modifiers restore their prior values. Keep a no-op plan stable.
func preserveManagedNoopPlan(req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var configured, planned, previous map[string]tftypes.Value
	if err := req.Config.Raw.As(&configured); err != nil {
		resp.Diagnostics.AddError("Invalid agent configuration", err.Error())
		return
	}
	if err := req.Plan.Raw.As(&planned); err != nil {
		resp.Diagnostics.AddError("Invalid agent plan", err.Error())
		return
	}
	if err := req.State.Raw.As(&previous); err != nil {
		resp.Diagnostics.AddError("Invalid agent state", err.Error())
		return
	}
	for name, attribute := range req.Plan.Schema.GetAttributes() {
		if attribute.IsComputed() && !attribute.IsOptional() {
			continue
		}
		if attribute.IsComputed() && configured[name].IsNull() {
			continue
		}
		if !planned[name].Equal(previous[name]) {
			return
		}
	}
	resp.Plan.Raw = req.State.Raw
}

func (r *AgentResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
