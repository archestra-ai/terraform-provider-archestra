package provider

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *AppResource) readTools(ctx context.Context, id uuid.UUID, diags *diag.Diagnostics) types.Set {
	result, err := r.client.GetAppToolsWithResponse(ctx, id)
	if err != nil {
		diags.AddError("App tool read failed", err.Error())
		return types.SetNull(agentInlineToolType)
	}
	if result.StatusCode() != 200 {
		appRequestError(diags, "App tool read", result.StatusCode())
		return types.SetNull(agentInlineToolType)
	}
	var rows []struct {
		ID     string          `json:"id"`
		Server json.RawMessage `json:"mcpServerId"`
		Mode   *string         `json:"credentialResolutionMode"`
	}
	if err := json.Unmarshal(result.Body, &rows); err != nil {
		diags.AddError("Invalid App tool response", err.Error())
		return types.SetNull(agentInlineToolType)
	}
	values := make([]agentInlineTool, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if !canonicalUUIDRegexp.MatchString(row.ID) || seen[row.ID] {
			diags.AddError("Invalid App tool identity", "The API returned an invalid or duplicate tool UUID.")
			return types.SetNull(agentInlineToolType)
		}
		seen[row.ID] = true
		if len(row.Server) == 0 || row.Mode == nil {
			diags.AddError("App assignment read contract unavailable", "The API must return stored server pins and credential modes. Upgrade Archestra to 1.4.0-rc.50 or later.")
			return types.SetNull(agentInlineToolType)
		}
		if *row.Mode != "static" && *row.Mode != "dynamic" && *row.Mode != "enterprise_managed" {
			diags.AddError("Unsupported App credential mode", "Upgrade the provider before editing this assignment.")
			return types.SetNull(agentInlineToolType)
		}
		var server *string
		if err := json.Unmarshal(row.Server, &server); err != nil || (server != nil && !canonicalUUIDRegexp.MatchString(*server)) {
			diags.AddError("Invalid App server pin", "The API did not return a canonical server UUID or explicit null.")
			return types.SetNull(agentInlineToolType)
		}
		values = append(values, agentInlineTool{ToolID: types.StringValue(row.ID), McpServerID: types.StringPointerValue(server), CredentialResolutionMode: types.StringValue(*row.Mode)})
	}
	value, d := types.SetValueFrom(ctx, agentInlineToolType, values)
	diags.Append(d...)
	return value
}

func (r *AppResource) writeTools(ctx context.Context, desired, observed *AppResourceModel, diags *diag.Diagnostics) bool {
	if desired.Tools.IsNull() || desired.Tools.IsUnknown() || desired.Tools.Equal(observed.Tools) {
		return true
	}
	id, err := uuid.Parse(observed.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid App UUID", err.Error())
		return false
	}
	var before, after []agentInlineTool
	diags.Append(observed.Tools.ElementsAs(ctx, &before, false)...)
	diags.Append(desired.Tools.ElementsAs(ctx, &after, false)...)
	if diags.HasError() {
		return false
	}
	previous := map[string]agentInlineTool{}
	for _, tool := range before {
		previous[tool.ToolID.ValueString()] = tool
	}
	requested := map[string]bool{}
	for _, tool := range after {
		key := tool.ToolID.ValueString()
		if requested[key] {
			diags.AddError("Duplicate App tool selection", "Assign each tool UUID once.")
			return false
		}
		requested[key] = true
	}
	// A rejected new assignment must not remove any existing extra tools.
	for _, tool := range after {
		key := tool.ToolID.ValueString()
		if prior, found := previous[key]; found && prior.McpServerID.Equal(tool.McpServerID) && prior.CredentialResolutionMode.Equal(tool.CredentialResolutionMode) {
			continue
		}
		toolID, err := uuid.Parse(key)
		if err != nil {
			diags.AddError("Invalid App tool UUID", err.Error())
			return false
		}
		var pin any
		if !tool.McpServerID.IsNull() {
			pin = tool.McpServerID.ValueString()
		}
		body, err := json.Marshal(map[string]any{"mcpServerId": pin, "credentialResolutionMode": tool.CredentialResolutionMode.ValueString()})
		if err != nil {
			diags.AddError("App assignment encoding failed", err.Error())
			return false
		}
		result, err := r.client.AssignToolToAppWithBodyWithResponse(ctx, id, toolID, "application/json", bytes.NewReader(body))
		if err != nil {
			diags.AddError("App tool assignment failed", err.Error())
			return false
		}
		if result.StatusCode() != 200 {
			appRequestError(diags, "App tool assignment; existing extra tools retained", result.StatusCode())
			return false
		}
	}
	for _, tool := range before {
		if requested[tool.ToolID.ValueString()] {
			continue
		}
		toolID, err := uuid.Parse(tool.ToolID.ValueString())
		if err != nil {
			diags.AddError("Invalid App tool UUID", err.Error())
			return false
		}
		result, err := r.client.UnassignToolFromAppWithResponse(ctx, id, toolID)
		if err != nil {
			diags.AddError("App tool removal failed", err.Error())
			return false
		}
		if result.StatusCode() != 200 && result.StatusCode() != 204 && result.StatusCode() != 404 {
			appRequestError(diags, "App tool removal", result.StatusCode())
			return false
		}
	}
	return true
}

// Explicit enable/unlock precedes content editing. Disable/lock follows it.
// Omitted settings never become an implicit authoring-gate bypass.
func (r *AppResource) applyLifecycle(ctx context.Context, desired, observed *AppResourceModel, before bool, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(observed.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid App UUID", err.Error())
		return false
	}
	if !desired.Enabled.IsNull() && !desired.Enabled.IsUnknown() && !desired.Enabled.Equal(observed.Enabled) && desired.Enabled.ValueBool() == before {
		var status int
		if desired.Enabled.ValueBool() {
			result, callErr := r.client.EnableAppWithResponse(ctx, id)
			err = callErr
			if result != nil {
				status = result.StatusCode()
			}
		} else {
			result, callErr := r.client.DisableAppWithResponse(ctx, id)
			err = callErr
			if result != nil {
				status = result.StatusCode()
			}
		}
		if err != nil {
			diags.AddError("App availability update failed", err.Error())
			return false
		}
		if status != 200 {
			appRequestError(diags, "App availability update", status)
			return false
		}
		observed.Enabled = desired.Enabled
	}
	if !desired.Locked.IsNull() && !desired.Locked.IsUnknown() && !desired.Locked.Equal(observed.Locked) && desired.Locked.ValueBool() != before {
		var status int
		if desired.Locked.ValueBool() {
			result, callErr := r.client.LockAppWithResponse(ctx, id)
			err = callErr
			if result != nil {
				status = result.StatusCode()
			}
		} else {
			result, callErr := r.client.UnlockAppWithResponse(ctx, id)
			err = callErr
			if result != nil {
				status = result.StatusCode()
			}
		}
		if err != nil {
			diags.AddError("App lock update failed", err.Error())
			return false
		}
		if status != 200 {
			appRequestError(diags, "App lock update", status)
			return false
		}
		observed.Locked = desired.Locked
	}
	return true
}
