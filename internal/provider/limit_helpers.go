package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

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

func (r *LimitResource) APIShape() any { return client.GetLimitResponse{} }

// KnownIntentionallySkipped: createdAt/updatedAt are audit timestamps;
// lastCleanup is a backend bookkeeping field tracking the limit cleanup
// scheduler — debug-only, not user-facing.
func (r *LimitResource) KnownIntentionallySkipped() []string {
	return []string{"createdAt", "updatedAt", "lastCleanup"}
}
