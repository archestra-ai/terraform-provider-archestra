package provider

import (
	"context"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var limitAttrSpec = []AttrSpec{
	{TFName: "reason", JSONName: "reason", Kind: Scalar},
	{TFName: "cleanup_interval", JSONName: "cleanupInterval", Kind: Scalar, OmitOnNull: true},
	{TFName: "labels", JSONName: "labels", Kind: Set, OmitOnNull: true, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
	{TFName: "entity_id", JSONName: "entityId", Kind: Scalar},
	{TFName: "entity_type", JSONName: "entityType", Kind: Scalar},
	{TFName: "limit_type", JSONName: "limitType", Kind: Scalar},
	{TFName: "limit_value", JSONName: "limitValue", Kind: Scalar},
	{TFName: "model", JSONName: "model", Kind: List},
	{TFName: "tool_name", JSONName: "toolName", Kind: Scalar},
	{TFName: "mcp_server_name", JSONName: "mcpServerName", Kind: Scalar},
}

func (r *LimitResource) AttrSpecs() []AttrSpec { return limitAttrSpec }

func (r *LimitResource) APIShape() any { return client.GetLimitResponse{} }

// KnownIntentionallySkipped: createdAt/updatedAt are audit timestamps;
// lastCleanup is a backend bookkeeping field tracking the limit cleanup
// scheduler — debug-only, not user-facing.
func (r *LimitResource) KnownIntentionallySkipped() []string {
	return []string{"createdAt", "updatedAt", "lastCleanup"}
}

func (r *LimitResource) refreshCurrentLimit(ctx context.Context, data *LimitResourceModel, diags *diag.Diagnostics) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Limit ID", "The API returned an invalid limit identifier.")
		return
	}
	result, err := r.client.GetLimitWithResponse(ctx, id)
	if err != nil {
		diags.AddError("Limit Refresh Failed", err.Error())
		return
	}
	if result.JSON200 == nil {
		diags.AddError("Limit Refresh Failed", "The saved limit could not be read after writing.")
		return
	}
	r.flattenCurrentLimit(ctx, data, result, diags)
}
func (r *LimitResource) flattenCurrentLimit(ctx context.Context, data *LimitResourceModel, result *client.GetLimitResponse, diags *diag.Diagnostics) {
	wire := result.JSON200
	data.Reason = types.StringPointerValue(wire.Reason)
	data.ThroughputUsage = types.ObjectNull(limitThroughputUsageAttrTypes)
	if wire.ThroughputUsage != nil {
		value, ds := types.ObjectValue(limitThroughputUsageAttrTypes, map[string]attr.Value{"current_usage": types.Float64Value(float64(wire.ThroughputUsage.CurrentUsage)), "resets_at": types.StringValue(wire.ThroughputUsage.ResetsAt)})
		diags.Append(ds...)
		data.ThroughputUsage = value
	}
	data.CleanupInterval = types.StringNull()
	if wire.CleanupInterval != nil {
		data.CleanupInterval = types.StringValue(string(*wire.CleanupInterval))
	}
	data.Labels = flattenAgentLabels(data.Labels, wire.Labels)
	usageModels := []LimitModelUsageModel{}
	if wire.ModelUsage != nil {
		usageModels = make([]LimitModelUsageModel, len(*wire.ModelUsage))
		for i, usage := range *wire.ModelUsage {
			usageModels[i] = LimitModelUsageModel{Model: types.StringValue(usage.Model), Cost: types.Float64Value(float64(usage.Cost)), TokensIn: types.Float64Value(float64(usage.TokensIn)), TokensOut: types.Float64Value(float64(usage.TokensOut))}
		}
	}
	value, ds := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{"model": types.StringType, "cost": types.Float64Type, "tokens_in": types.Float64Type, "tokens_out": types.Float64Type}}, usageModels)
	diags.Append(ds...)
	data.ModelUsage = value

}

var limitThroughputUsageAttrTypes = map[string]attr.Type{"current_usage": types.Float64Type, "resets_at": types.StringType}
