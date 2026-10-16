package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type agentInlineTool struct {
	ToolID                   types.String `tfsdk:"tool_id"`
	McpServerID              types.String `tfsdk:"mcp_server_id"`
	CredentialResolutionMode types.String `tfsdk:"credential_resolution_mode"`
}
type agentToolAssignment struct {
	ID          uuid.UUID
	ToolID      string
	McpServerID *uuid.UUID
	Mode        string
}

var agentInlineToolType = types.ObjectType{AttrTypes: map[string]attr.Type{"tool_id": types.StringType, "mcp_server_id": types.StringType, "credential_resolution_mode": types.StringType}}

func agentToolsAttribute() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{Optional: true, Computed: true, MarkdownDescription: "Complete explicit MCP tool assignments; omit to preserve or set an empty set to clear, preserving local/remote subagents and platform-required recovery tools.", Validators: []validator.Set{setvalidator.SizeAtMost(5000)}, PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"tool_id":                    schema.StringAttribute{Required: true, MarkdownDescription: "Tool UUID.", Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical tool UUID.")}},
		"mcp_server_id":              schema.StringAttribute{Optional: true, MarkdownDescription: "Backing MCP server UUID; omit to leave the assignment unpinned.", Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical server UUID.")}},
		"credential_resolution_mode": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("static"), MarkdownDescription: "Credential mode: static, dynamic, or enterprise_managed.", Validators: []validator.String{stringvalidator.OneOf("static", "dynamic", "enterprise_managed")}},
	}}}
}
func (r *AgentResource) ordinaryToolAssignments(ctx context.Context, id uuid.UUID, diags *diag.Diagnostics) ([]agentToolAssignment, map[string]bool) {
	agent, err := r.client.GetAgentWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read tool ownership", 0, err)
		return nil, nil
	}
	if agent.JSON200 == nil {
		agentRelationshipError(diags, "Read tool ownership", agent.StatusCode(), nil)
		return nil, nil
	}
	if agent.JSON200.Id != id {
		diags.AddError("Unexpected tool ownership scope", "The API returned another agent while reading tool ownership.")
		return nil, nil
	}
	delegated := map[string]bool{}
	for _, tool := range agent.JSON200.Tools {
		delegated[tool.Id.String()] = tool.DelegateToAgentId != nil || tool.DelegateToA2aConnectionId != nil || (tool.CatalogId != nil && requiredPlatformTool(tool.CatalogId.String(), tool.Name))
	}
	limit, offset := 100, 0
	out := []agentToolAssignment{}
	seen := map[string]bool{}
	for {
		response, err := r.client.GetAllAgentToolsWithResponse(ctx, &client.GetAllAgentToolsParams{AgentId: &id, Limit: &limit, Offset: &offset})
		if err != nil {
			agentRelationshipError(diags, "Read agent tools", 0, err)
			return nil, nil
		}
		if response.JSON200 == nil {
			agentRelationshipError(diags, "Read agent tools", response.StatusCode(), nil)
			return nil, nil
		}
		for _, row := range response.JSON200.Data {
			if row.Agent.Id != id.String() {
				diags.AddError("Unexpected tool assignment scope", "The API returned a tool assignment for another agent.")
				return nil, nil
			}
			isDelegated, known := delegated[row.Tool.Id]
			if !known {
				diags.AddError("Incomplete tool ownership snapshot", "The agent and assignment listings changed concurrently or omit an assignment. Refresh before managing the tool set.")
				return nil, nil
			}
			if isDelegated {
				continue
			}
			if seen[row.Tool.Id] {
				diags.AddError("Duplicate tool assignment", "The API returned more than one assignment for the same tool.")
				return nil, nil
			}
			seen[row.Tool.Id] = true
			out = append(out, agentToolAssignment{ID: row.Id, ToolID: row.Tool.Id, McpServerID: row.McpServerId, Mode: string(row.CredentialResolutionMode)})
		}
		if !response.JSON200.Pagination.HasNext {
			break
		}
		if len(response.JSON200.Data) == 0 {
			diags.AddError("Invalid tool pagination", "The API returned an empty page while reporting more assignments.")
			return nil, nil
		}
		offset += limit
	}
	return out, delegated
}
func (r *AgentResource) readTools(ctx context.Context, data *AgentResourceModel, diags *diag.Diagnostics) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid agent ID", err.Error())
		return
	}
	rows, _ := r.ordinaryToolAssignments(ctx, id, diags)
	if diags.HasError() {
		return
	}
	values := make([]agentInlineTool, 0, len(rows))
	for _, row := range rows {
		var server *string
		if row.McpServerID != nil {
			v := row.McpServerID.String()
			server = &v
		}
		values = append(values, agentInlineTool{ToolID: types.StringValue(row.ToolID), McpServerID: types.StringPointerValue(server), CredentialResolutionMode: types.StringValue(row.Mode)})
	}
	value, d := types.SetValueFrom(ctx, agentInlineToolType, values)
	diags.Append(d...)
	data.Tools = value
	excluded, err := r.client.GetAgentToolExclusionsWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read excluded tools", 0, err)
		return
	}
	if excluded.JSON200 == nil {
		agentRelationshipError(diags, "Read excluded tools", excluded.StatusCode(), nil)
		return
	}
	ids := make([]string, 0, len(excluded.JSON200.ExcludedToolIds))
	for _, toolID := range excluded.JSON200.ExcludedToolIds {
		ids = append(ids, toolID.String())
	}
	data.ExcludedToolIDs = agentSet(ctx, ids, diags)
}
func (r *AgentResource) writeTools(ctx context.Context, plan, prior *AgentResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	id, err := uuid.Parse(plan.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid agent ID", err.Error())
		return
	}
	if agentConfigured(ctx, config, "tools", diags) && !plan.Tools.Equal(prior.Tools) {
		var desired []agentInlineTool
		diags.Append(plan.Tools.ElementsAs(ctx, &desired, false)...)
		if diags.HasError() {
			return
		}
		byID := map[string]agentInlineTool{}
		for _, tool := range desired {
			key := tool.ToolID.ValueString()
			if _, exists := byID[key]; exists {
				diags.AddError("Duplicate tool selection", "Assign each tool only once, with one credential mode and backing server.")
				return
			}
			byID[key] = tool
		}
		current, reserved := r.ordinaryToolAssignments(ctx, id, diags)
		if diags.HasError() {
			return
		}
		for key := range byID {
			if reserved[key] {
				diags.AddError("Reserved tool selection", "Use subagent_ids or remote_agent_ids for subagents. Platform-required recovery tools are managed automatically; omit them from tools.")
				return
			}
		}
		observed := map[string]agentToolAssignment{}
		for _, row := range current {
			observed[row.ToolID] = row
		}
		assignments := []map[string]any{}
		requested := map[string]bool{}
		for _, tool := range desired {
			key := tool.ToolID.ValueString()
			before, found := observed[key]
			var server any
			serverValue := ""
			if !tool.McpServerID.IsNull() {
				serverValue = tool.McpServerID.ValueString()
				server = serverValue
			}
			previousServer := ""
			if before.McpServerID != nil {
				previousServer = before.McpServerID.String()
			}
			if found && before.Mode == tool.CredentialResolutionMode.ValueString() && previousServer == serverValue {
				continue
			}
			assignments = append(assignments, map[string]any{"agentId": id.String(), "toolId": key, "mcpServerId": server, "credentialResolutionMode": tool.CredentialResolutionMode.ValueString()})
			requested[key] = true
		}
		if len(assignments) > 0 {
			body, err := json.Marshal(map[string]any{"assignments": assignments})
			if err != nil {
				diags.AddError("Tool selection encoding failed", err.Error())
				return
			}
			result, err := r.client.BulkAssignToolsWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
			if err != nil {
				agentRelationshipError(diags, "Assign agent tools", 0, err)
				return
			}
			if result.JSON200 == nil {
				agentRelationshipError(diags, "Assign agent tools", result.StatusCode(), nil)
				return
			}
			if len(result.JSON200.Failed) > 0 {
				first := result.JSON200.Failed[0]
				if first.AgentId != id.String() || !requested[first.ToolId] {
					diags.AddError("Unexpected tool assignment rejection", "The API returned a rejection outside this change; existing assignments were retained.")
					return
				}
				diags.AddError("Tool selection rejected", fmt.Sprintf("The API rejected %d assignment(s). Tool %s: %s. Existing extra assignments were retained.", len(result.JSON200.Failed), first.ToolId, first.Error))
				return
			}
			acknowledged := map[string]bool{}
			for _, row := range result.JSON200.Succeeded {
				if row.AgentId != id.String() || !requested[row.ToolId] {
					diags.AddError("Unexpected tool assignment acknowledgement", "The API acknowledged an assignment outside this change.")
					return
				}
				acknowledged[row.ToolId] = true
			}
			for _, row := range result.JSON200.Duplicates {
				if row.AgentId != id.String() || !requested[row.ToolId] {
					diags.AddError("Unexpected tool assignment acknowledgement", "The API acknowledged an assignment outside this change.")
					return
				}
				acknowledged[row.ToolId] = true
			}
			if len(acknowledged) != len(requested) {
				diags.AddError("Incomplete tool assignment acknowledgement", "The API did not confirm every requested assignment; extra assignments were retained.")
				return
			}
		}
		for _, row := range current {
			if _, keep := byID[row.ToolID]; keep {
				continue
			}
			toolID, err := uuid.Parse(row.ToolID)
			if err != nil {
				diags.AddError("Invalid tool identity", err.Error())
				return
			}
			result, err := r.client.UnassignToolFromAgentWithResponse(ctx, id, toolID)
			if err != nil {
				agentRelationshipError(diags, "Remove agent tool", 0, err)
				return
			}
			if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
				var details struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				_ = json.Unmarshal(result.Body, &details)
				diags.AddError("Agent tool removal rejected", fmt.Sprintf("Tool %s: API status %d: %s", row.ToolID, result.StatusCode(), details.Error.Message))
				return
			}
		}
	}
	if diags.HasError() || !agentConfigured(ctx, config, "excluded_tool_ids", diags) || plan.ExcludedToolIDs.Equal(prior.ExcludedToolIDs) {
		return
	}
	helper := AgentToolExclusionsResource{client: r.client}
	helper.replaceExclusions(ctx, &AgentToolExclusionsResourceModel{AgentID: plan.ID, ExcludedToolIds: plan.ExcludedToolIDs}, diags)
}
func validateInlineTools(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	var tools types.Set
	diags.Append(config.GetAttribute(ctx, path.Root("tools"), &tools)...)
	if tools.IsNull() || tools.IsUnknown() || diags.HasError() {
		return
	}
	var values []agentInlineTool
	diags.Append(tools.ElementsAs(ctx, &values, true)...)
	seen := map[string]bool{}
	for _, tool := range values {
		if tool.ToolID.IsUnknown() {
			continue
		}
		id := tool.ToolID.ValueString()
		if seen[id] {
			diags.AddError("Duplicate tool selection", "Use one assignment per tool UUID.")
		}
		seen[id] = true
	}
}

// The product preserves these platform-required recovery assignments even when
// its tool picker is cleared. Match the stable catalog ID, not branded prefixes.
func requiredPlatformTool(catalogID, name string) bool {
	if catalogID != "00000000-0000-4000-8000-000000000001" {
		return false
	}
	shortName := name
	if index := strings.LastIndex(name, "__"); index > 0 {
		shortName = name[index+2:]
	}
	switch shortName {
	case "get_remedy_plans", "execute_remedy_plan", "list_peer_messages", "read_peer_message", "yell":
		return true
	}
	return false
}
