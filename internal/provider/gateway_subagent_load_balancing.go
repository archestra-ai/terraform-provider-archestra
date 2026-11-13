package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The API fills the same defaults when it stores the setting, so a partial
// block plans and reads back identically.

var subagentLoadBalancingAttrTypes = map[string]attr.Type{
	"enabled":          types.BoolType,
	"algorithm":        types.StringType,
	"weights":          types.MapType{ElemType: types.Int64Type},
	"retries":          types.Int64Type,
	"fallback_enabled": types.BoolType,
}

var gatewaySubagentLoadBalancingAttrSpec = AttrSpec{
	TFName: "subagent_load_balancing", JSONName: "subagentLoadBalancing", Kind: AtomicObject,
	Children: []AttrSpec{
		{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
		{TFName: "algorithm", JSONName: "algorithm", Kind: Scalar},
		{TFName: "weights", JSONName: "weights", Kind: Map},
		{TFName: "retries", JSONName: "retries", Kind: Scalar},
		{TFName: "fallback_enabled", JSONName: "fallbackEnabled", Kind: Scalar},
	},
}

func gatewaySubagentLoadBalancingAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		MarkdownDescription: "How the gateway spreads delegated requests across its assigned subagents and retries failures. " +
			"Omit for the default (no load balancing); removing it later clears the setting. Requires API 1.5.0-rc.23 or later.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Whether requests are distributed across the assigned subagents (default `false`).",
			},
			"algorithm": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("round_robin"),
				MarkdownDescription: "`round_robin` (default), `weighted` or `random`.",
				Validators:          []validator.String{stringvalidator.OneOf("round_robin", "weighted", "random")},
			},
			"weights": schema.MapAttribute{
				Optional: true, ElementType: types.Int64Type,
				MarkdownDescription: "Weights from 1 to 1000 keyed by subagent UUID, used by `weighted`; unlisted subagents weigh 1.",
				Validators: []validator.Map{
					mapvalidator.KeysAre(stringvalidator.RegexMatches(canonicalUUIDRegexp, "Key weights by lowercase canonical subagent UUID.")),
					mapvalidator.ValueInt64sAre(int64validator.Between(1, 1000)),
				},
			},
			"retries": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(1),
				MarkdownDescription: "Retries on the same subagent after a retriable failure, 0 to 3 (default `1`).",
				Validators:          []validator.Int64{int64validator.Between(0, 3)},
			},
			"fallback_enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				MarkdownDescription: "Whether a failed request falls back to another assigned subagent (default `true`).",
			},
		},
	}
}

// subagentLoadBalancingFromResponse reads the stored setting; null when the
// gateway has none, and on releases without it.
func subagentLoadBalancingFromResponse(body []byte, diags *diag.Diagnostics) types.Object {
	var wire struct {
		Config *struct {
			Enabled         bool             `json:"enabled"`
			Algorithm       string           `json:"algorithm"`
			Weights         map[string]int64 `json:"weights"`
			Retries         int64            `json:"retries"`
			FallbackEnabled bool             `json:"fallbackEnabled"`
		} `json:"subagentLoadBalancing"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		diags.AddError("Invalid Gateway Response", "Unexpected subagentLoadBalancing value: "+err.Error())
		return types.ObjectNull(subagentLoadBalancingAttrTypes)
	}
	if wire.Config == nil {
		return types.ObjectNull(subagentLoadBalancingAttrTypes)
	}
	c := wire.Config
	weights := types.MapNull(types.Int64Type)
	if c.Weights != nil {
		var d diag.Diagnostics
		weights, d = types.MapValueFrom(context.Background(), types.Int64Type, c.Weights)
		diags.Append(d...)
	}
	out, d := types.ObjectValue(subagentLoadBalancingAttrTypes, map[string]attr.Value{
		"enabled":          types.BoolValue(c.Enabled),
		"algorithm":        types.StringValue(c.Algorithm),
		"weights":          weights,
		"retries":          types.Int64Value(c.Retries),
		"fallback_enabled": types.BoolValue(c.FallbackEnabled),
	})
	diags.Append(d...)
	return out
}
