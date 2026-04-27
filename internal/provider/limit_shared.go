package provider

var limitAttrSpec = []AttrSpec{
	{TFName: "entity_id", JSONName: "entityId", Kind: Scalar},
	{TFName: "entity_type", JSONName: "entityType", Kind: Scalar},
	{TFName: "limit_type", JSONName: "limitType", Kind: Scalar},
	{TFName: "limit_value", JSONName: "limitValue", Kind: Scalar},
	{TFName: "model", JSONName: "model", Kind: List},
	{TFName: "tool_name", JSONName: "toolName", Kind: Scalar},
	{TFName: "mcp_server_name", JSONName: "mcpServerName", Kind: Scalar},
}

func (r *LimitResource) AttrSpecs() []AttrSpec { return limitAttrSpec }
