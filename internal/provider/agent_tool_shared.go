package provider

// agentToolAttrSpec covers the agent-tool body. agent_id and tool_id ride in
// the URL path (RequiresReplace), so they're Synthetic from the merge-patch
// point of view — see the schema's RequiresReplace plan modifier.
var agentToolAttrSpec = []AttrSpec{
	{TFName: "agent_id", Kind: Synthetic},
	{TFName: "tool_id", Kind: Synthetic},
	{TFName: "mcp_server_id", JSONName: "mcpServerId", Kind: Scalar},
	{TFName: "credential_resolution_mode", JSONName: "credentialResolutionMode", Kind: Scalar},
}

func (r *AgentToolResource) AttrSpecs() []AttrSpec { return agentToolAttrSpec }
