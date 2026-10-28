package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Project saved state onto the current schema without touching remote objects.
// Retained IDs and credential values must survive a schema-only upgrade.
func schemaStateUpgrade(r schemaResource) resource.StateUpgrader {
	return resource.StateUpgrader{StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
		if req.RawState == nil || len(req.RawState.JSON) == 0 {
			resp.Diagnostics.AddError("Missing Previous State", "Expected JSON resource state.")
			return
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(req.RawState.JSON, &values); err != nil {
			resp.Diagnostics.AddError("Invalid Previous State", "Unable to decode the saved resource state.")
			return
		}
		var id string
		if err := json.Unmarshal(values["id"], &id); err != nil || id == "" {
			resp.Diagnostics.AddError("Invalid Previous State", "Existing string ID is required; restore the state backup before retrying.")
			return
		}
		var current resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &current)
		resp.Diagnostics.Append(current.Diagnostics...)
		if resp.Diagnostics.HasError() {
			return
		}
		kind := current.Schema.Type().TerraformType(ctx)
		encoded, err := projectStateJSON(req.RawState.JSON, kind)
		if err != nil {
			resp.Diagnostics.AddError("State Upgrade Failed", "Previous state has incompatible values; restore the state backup before retrying.")
			return
		}
		upgraded, err := (tfprotov6.RawState{JSON: encoded}).Unmarshal(kind)
		if err != nil {
			resp.Diagnostics.AddError("State Upgrade Failed", "Previous state has incompatible values; restore the state backup before retrying.")
			return
		}
		resp.State.Schema = current.Schema
		resp.State.Raw = upgraded
	}}
}

func (r *MCPServerResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: schemaStateUpgrade(r)}
}
