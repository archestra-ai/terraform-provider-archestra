package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

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

func (r *AgentToolResource) APIShape() any { return client.GetAllAgentToolsResponse{} }

// KnownIntentionallySkipped: `agent` and `tool` are nested objects in the
// response (`{id, name}` each). The provider flattens them into the path-
// keyed `agent_id` / `tool_id` synthetics — there's no value in surfacing
// the whole nested object. `pagination` is list-endpoint metadata, not a
// per-row field. `createdAt` / `updatedAt` are audit timestamps not
// surfaced today (would be fine to add as Computed-only later).
func (r *AgentToolResource) KnownIntentionallySkipped() []string {
	return []string{"agent", "tool", "pagination", "createdAt", "updatedAt"}
}
