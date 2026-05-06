package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// Tool invocation policies and trusted data policies share a wire shape:
// `{toolId, conditions: [{key, operator, value}], action, reason?}`. The
// previous schema only exposed one condition by collapsing it into three
// scalars (argument_name | attribute_path / operator / value). The redesign
// raises that to ListNested matching the wire 1-to-1, so callers can declare
// multiple conditions per policy.

var toolInvocationPolicyAttrSpec = []AttrSpec{
	{TFName: "tool_id", JSONName: "toolId", Kind: Scalar},
	{TFName: "conditions", JSONName: "conditions", Kind: List, Children: []AttrSpec{
		{TFName: "key", JSONName: "key", Kind: Scalar},
		{TFName: "operator", JSONName: "operator", Kind: Scalar},
		{TFName: "value", JSONName: "value", Kind: Scalar},
	}},
	{TFName: "action", JSONName: "action", Kind: Scalar},
	{TFName: "reason", JSONName: "reason", Kind: Scalar},
}

func (r *ToolInvocationPolicyResource) AttrSpecs() []AttrSpec {
	return toolInvocationPolicyAttrSpec
}

func (r *ToolInvocationPolicyResource) APIShape() any {
	return client.GetToolInvocationPolicyResponse{}
}

// KnownIntentionallySkipped: createdAt/updatedAt are audit timestamps.
func (r *ToolInvocationPolicyResource) KnownIntentionallySkipped() []string {
	return []string{"createdAt", "updatedAt"}
}

var trustedDataPolicyAttrSpec = []AttrSpec{
	{TFName: "tool_id", JSONName: "toolId", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "conditions", JSONName: "conditions", Kind: List, Children: []AttrSpec{
		{TFName: "key", JSONName: "key", Kind: Scalar},
		{TFName: "operator", JSONName: "operator", Kind: Scalar},
		{TFName: "value", JSONName: "value", Kind: Scalar},
	}},
	{TFName: "action", JSONName: "action", Kind: Scalar},
}

func (r *TrustedDataPolicyResource) AttrSpecs() []AttrSpec {
	return trustedDataPolicyAttrSpec
}

func (r *TrustedDataPolicyResource) APIShape() any {
	return client.GetTrustedDataPolicyResponse{}
}

// KnownIntentionallySkipped: createdAt/updatedAt are audit timestamps.
func (r *TrustedDataPolicyResource) KnownIntentionallySkipped() []string {
	return []string{"createdAt", "updatedAt"}
}
