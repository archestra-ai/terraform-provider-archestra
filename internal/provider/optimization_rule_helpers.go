package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// optimizationRuleAttrSpec covers the optimization rule wire body.
//
// `conditions` rides through MergePatch as a List<Object>, but the wire format
// is a polymorphic discriminated union: each plan-side row produces one or two
// wire entries — `{maxLength: N}` and/or `{hasTools: bool}`. The Encoder fans
// the encoded list out into the union shape after MergePatch builds it.
//
// (The schema-side cleanup that makes the HCL match the wire 1:1 is tracked
// as Phase 3 #20.)
var optimizationRuleAttrSpec = []AttrSpec{
	{TFName: "entity_id", JSONName: "entityId", Kind: Scalar},
	{TFName: "entity_type", JSONName: "entityType", Kind: Scalar},
	{TFName: "llm_provider", JSONName: "provider", Kind: Scalar},
	{TFName: "target_model", JSONName: "targetModel", Kind: Scalar},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
	{TFName: "conditions", JSONName: "conditions", Kind: List, Encoder: encodeOptimizationConditions, Children: []AttrSpec{
		{TFName: "max_length", JSONName: "maxLength", Kind: Scalar},
		{TFName: "has_tools", JSONName: "hasTools", Kind: Scalar},
	}},
}

// encodeOptimizationConditions takes the post-encoded `[{maxLength?, hasTools?}]`
// list and explodes each entry into one wire object per non-null sub-field —
// the polymorphic union shape the backend expects.
func encodeOptimizationConditions(v any) any {
	in, ok := v.([]any)
	if !ok {
		return v
	}
	out := make([]map[string]any, 0, len(in))
	for _, item := range in {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if mv, ok := entry["maxLength"]; ok && mv != nil {
			out = append(out, map[string]any{"maxLength": mv})
		}
		if hv, ok := entry["hasTools"]; ok && hv != nil {
			out = append(out, map[string]any{"hasTools": hv})
		}
	}
	return out
}

func (r *OptimizationRuleResource) AttrSpecs() []AttrSpec { return optimizationRuleAttrSpec }

func (r *OptimizationRuleResource) APIShape() any { return client.GetOptimizationRulesResponse{} }

// KnownIntentionallySkipped: createdAt/updatedAt are audit timestamps.
func (r *OptimizationRuleResource) KnownIntentionallySkipped() []string {
	return []string{"createdAt", "updatedAt"}
}
