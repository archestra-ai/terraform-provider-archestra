package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var _ resource.ResourceWithUpgradeState = &LLMProviderApiKeyResource{}

// UpgradeState preserves the remote key and its sensitive state while renaming
// the misleading v2 attribute. This performs no API writes or key rotation.
func (r *LLMProviderApiKeyResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: {StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
		if req.RawState == nil || req.RawState.JSON == nil {
			resp.Diagnostics.AddError("Unsupported Key State", "Expected JSON state from provider v2.")
			return
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(req.RawState.JSON, &values); err != nil {
			resp.Diagnostics.AddError("Invalid Key State", "Unable to decode the previous key state.")
			return
		}
		var id string
		if err := json.Unmarshal(values["id"], &id); err != nil || id == "" {
			resp.Diagnostics.AddError("Invalid Key State", "Previous key state must contain its existing string ID.")
			return
		}
		if old, ok := values["is_organization_default"]; ok {
			values["is_primary"] = old
			delete(values, "is_organization_default")
		}
		values["is_agent_key"] = json.RawMessage("null")
		var current resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &current)
		resp.Diagnostics.Append(current.Diagnostics...)
		if resp.Diagnostics.HasError() {
			return
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			resp.Diagnostics.AddError("Key State Upgrade Failed", "Unable to encode the upgraded state.")
			return
		}
		upgraded, err := (tfprotov6.RawState{JSON: encoded}).Unmarshal(current.Schema.Type().TerraformType(ctx))
		if err != nil {
			resp.Diagnostics.AddError("Key State Upgrade Failed", "Previous key state does not match the supported v2 schema. Restore the state backup and inspect the provider version before retrying.")
			return
		}
		resp.State.Schema = current.Schema
		resp.State.Raw = upgraded
	}}}
}
