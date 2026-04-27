package provider

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
