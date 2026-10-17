package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type agentInlinePolicy struct {
	Mode     types.String `tfsdk:"mode"`
	Allowed  types.Set    `tfsdk:"allowed"`
	Excluded types.Set    `tfsdk:"excluded"`
}

var agentInlinePolicyType = types.ObjectType{AttrTypes: map[string]attr.Type{"mode": types.StringType, "allowed": types.SetType{ElemType: skillReferenceType}, "excluded": types.SetType{ElemType: skillReferenceType}}}

func agentIDSetAttribute(description string, maximum int) schema.SetAttribute {
	validators := []validator.Set{setvalidator.ValueStringsAre(stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use lowercase canonical UUIDs."))}
	if maximum > 0 {
		validators = append(validators, setvalidator.SizeAtMost(maximum))
	}
	return schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Validators: validators, PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}, MarkdownDescription: description}
}
func agentRelationshipAttributes() map[string]schema.Attribute {
	ids := func(description string) schema.SetAttribute { return agentIDSetAttribute(description, 0) }
	references := func(description string) schema.SetNestedAttribute {
		return schema.SetNestedAttribute{Required: true, MarkdownDescription: description, Validators: []validator.Set{setvalidator.SizeAtMost(1000)}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"source":        schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("native", "external_mcp", "plugin")}, MarkdownDescription: "Skill source kind."},
			"skill_id":      schema.StringAttribute{Optional: true, MarkdownDescription: "Native skill UUID."},
			"mcp_server_id": schema.StringAttribute{Optional: true, MarkdownDescription: "MCP server UUID for an external skill."},
			"uri":           schema.StringAttribute{Optional: true, MarkdownDescription: "External MCP skill URI."},
			"plugin_id":     schema.StringAttribute{Optional: true, MarkdownDescription: "Plugin UUID."},
			"skill_path":    schema.StringAttribute{Optional: true, MarkdownDescription: "Skill path within the plugin."},
		}}}
	}
	return map[string]schema.Attribute{
		"subagent_ids":           ids("Local agents available as subagents; omit to preserve assignments or set an empty set to clear them."),
		"remote_agent_ids":       agentIDSetAttribute("Registered remote-agent UUIDs assigned to this agent; omit to preserve or set an empty set to clear.", 500),
		"excluded_subagent_ids":  ids("Local agents excluded from automatic subagent access; omit to preserve or set an empty set to clear."),
		"excluded_connector_ids": ids("Knowledge connectors excluded from automatic knowledge access; omit to preserve or set an empty set to clear."),
		"skills": schema.SingleNestedAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Skill activation mode and complete allowed/excluded references; omit to preserve the current policy.", Attributes: map[string]schema.Attribute{
			"mode":     schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("all", "manual")}, MarkdownDescription: "Automatic access to all skills, or an explicit manual selection."},
			"allowed":  references("Explicitly allowed skills; an empty set clears the selection."),
			"excluded": references("Skills excluded from automatic access; an empty set clears exclusions."),
		}},
		"skill_policy_hidden_allowed_count":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Allowed skill references hidden or unavailable to the current caller."},
		"skill_policy_hidden_excluded_count": schema.Int64Attribute{Computed: true, MarkdownDescription: "Excluded skill references hidden or unavailable to the current caller."},
	}
}
func agentRelationshipAttrSpecs() []AttrSpec {
	return []AttrSpec{
		{TFName: "subagent_ids", Kind: Synthetic}, {TFName: "remote_agent_ids", Kind: Synthetic}, {TFName: "excluded_subagent_ids", Kind: Synthetic}, {TFName: "excluded_connector_ids", Kind: Synthetic}, {TFName: "skills", Kind: Synthetic},
	}
}
func agentSet(ctx context.Context, values []string, diags *diag.Diagnostics) types.Set {
	if values == nil {
		values = []string{}
	}
	set, d := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	return set
}
func agentConfigured(ctx context.Context, config tfsdk.Config, name string, diags *diag.Diagnostics) bool {
	if name == "skills" {
		var value types.Object
		diags.Append(config.GetAttribute(ctx, path.Root(name), &value)...)
		return !value.IsNull() && !value.IsUnknown()
	}
	var value types.Set
	diags.Append(config.GetAttribute(ctx, path.Root(name), &value)...)
	return !value.IsNull() && !value.IsUnknown()
}
func agentRelationshipError(diags *diag.Diagnostics, operation string, status int, err error) {
	if err != nil {
		diags.AddError("Agent relationship API error", fmt.Sprintf("%s failed: %s", operation, err))
		return
	}
	diags.AddError("Agent relationship API error", fmt.Sprintf("%s returned status %d; existing agent state is retained.", operation, status))
}
func (r *AgentResource) readAssignments(ctx context.Context, data *AgentResourceModel, diags *diag.Diagnostics) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid agent ID", err.Error())
		return
	}
	local, err := r.client.GetAgentDelegationsWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read subagents", 0, err)
		return
	}
	if local.JSON200 == nil {
		agentRelationshipError(diags, "Read subagents", local.StatusCode(), nil)
		return
	}
	values := make([]string, 0, len(*local.JSON200))
	for _, item := range *local.JSON200 {
		values = append(values, item.Id.String())
	}
	data.SubagentIDs = agentSet(ctx, values, diags)
	remote, err := r.client.GetAgentA2aDelegationsWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read remote subagents", 0, err)
		return
	}
	if remote.JSON200 == nil {
		agentRelationshipError(diags, "Read remote subagents", remote.StatusCode(), nil)
		return
	}
	values = values[:0]
	for _, item := range *remote.JSON200 {
		values = append(values, item.RemoteAgentId.String())
	}
	data.RemoteAgentIDs = agentSet(ctx, values, diags)
	exclusions, err := r.client.GetAgentSubagentExclusionsWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read subagent exclusions", 0, err)
		return
	}
	if exclusions.JSON200 == nil {
		agentRelationshipError(diags, "Read subagent exclusions", exclusions.StatusCode(), nil)
		return
	}
	values = values[:0]
	for _, item := range exclusions.JSON200.ExcludedSubagentIds {
		values = append(values, item.String())
	}
	data.ExcludedSubagentIDs = agentSet(ctx, values, diags)
	knowledge, err := r.client.GetAgentKnowledgeSourceExclusionsWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read knowledge exclusions", 0, err)
		return
	}
	if knowledge.JSON200 == nil {
		agentRelationshipError(diags, "Read knowledge exclusions", knowledge.StatusCode(), nil)
		return
	}
	values = values[:0]
	for _, item := range knowledge.JSON200.ExcludedConnectorIds {
		values = append(values, item.String())
	}
	data.ExcludedConnectorIDs = agentSet(ctx, values, diags)
}
func (r *AgentResource) readInlineSkillPolicy(ctx context.Context, data *AgentResourceModel, diags *diag.Diagnostics) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid agent ID", err.Error())
		return
	}
	policy, err := r.client.GetAgentActivationSkillPolicyWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read skill policy", 0, err)
		return
	}
	if policy.JSON200 == nil {
		agentRelationshipError(diags, "Read skill policy", policy.StatusCode(), nil)
		return
	}
	var wire skillPolicyWire
	if err := json.Unmarshal(policy.Body, &wire); err != nil {
		diags.AddError("Invalid skill policy", err.Error())
		return
	}
	model := AgentSkillPolicyResourceModel{AgentID: data.ID}
	model.populate(ctx, &wire, diags)
	inline := agentInlinePolicy{Mode: model.Mode, Allowed: model.AllowedReferences, Excluded: model.ExcludedReferences}
	value, d := types.ObjectValueFrom(ctx, agentInlinePolicyType.AttrTypes, &inline)
	diags.Append(d...)
	data.SkillPolicy = value
	data.SkillPolicyHiddenAllowedCount = types.Int64Value(int64(wire.HiddenAllowedCount))
	data.SkillPolicyHiddenExcludedCount = types.Int64Value(int64(wire.HiddenExcludedCount))
	data.ActivationSkillPolicyRevision = model.Revision
}
func (r *AgentResource) writeAssignments(ctx context.Context, plan, prior *AgentResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	id, err := uuid.Parse(plan.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid agent ID", err.Error())
		return
	}
	changes := []struct {
		name            string
		desired, before types.Set
		write           func() bool
	}{
		{"subagent_ids", plan.SubagentIDs, prior.SubagentIDs, func() bool {
			var raw []string
			diags.Append(plan.SubagentIDs.ElementsAs(ctx, &raw, false)...)
			if diags.HasError() {
				return false
			}
			ids := make([]uuid.UUID, 0, len(raw))
			for _, value := range raw {
				parsed, err := uuid.Parse(value)
				if err != nil || parsed == id {
					diags.AddError("Invalid subagent", "Select existing local agents other than this agent.")
					return false
				}
				ids = append(ids, parsed)
			}
			result, err := r.client.SyncAgentDelegationsWithResponse(ctx, id, client.SyncAgentDelegationsJSONRequestBody{TargetAgentIds: ids})
			if err != nil {
				agentRelationshipError(diags, "Assign subagents", 0, err)
				return false
			}
			if result.JSON200 == nil {
				agentRelationshipError(diags, "Assign subagents", result.StatusCode(), nil)
				return false
			}
			return true
		}},
		{"remote_agent_ids", plan.RemoteAgentIDs, prior.RemoteAgentIDs, func() bool {
			var remoteIDs []string
			diags.Append(plan.RemoteAgentIDs.ElementsAs(ctx, &remoteIDs, false)...)
			if diags.HasError() {
				return false
			}
			connections := make([]string, 0, len(remoteIDs))
			for _, remoteID := range remoteIDs {
				remoteUUID, err := uuid.Parse(remoteID)
				if err != nil {
					diags.AddError("Invalid remote agent", err.Error())
					return false
				}
				remote, err := r.client.GetA2aRemoteAgentWithResponse(ctx, remoteUUID)
				if err != nil {
					agentRelationshipError(diags, "Resolve remote agent", 0, err)
					return false
				}
				if remote.JSON200 == nil {
					agentRelationshipError(diags, "Resolve remote agent", remote.StatusCode(), nil)
					return false
				}
				if remote.JSON200.Id != remoteUUID || remote.JSON200.Connection.RemoteAgentId != remoteUUID {
					diags.AddError("Unexpected remote agent identity", "The API returned a connection for another remote agent.")
					return false
				}
				connections = append(connections, remote.JSON200.Connection.Id.String())
			}
			connectionIDs := agentSet(ctx, connections, diags)
			helper := AgentRemoteDelegationsResource{client: r.client}
			return helper.replaceConnections(ctx, &AgentRemoteDelegationsResourceModel{AgentID: plan.ID, ConnectionIds: connectionIDs}, diags)
		}},
		{"excluded_subagent_ids", plan.ExcludedSubagentIDs, prior.ExcludedSubagentIDs, func() bool {
			helper := AgentSubagentExclusionsResource{client: r.client}
			return helper.replaceExclusions(ctx, &AgentSubagentExclusionsResourceModel{AgentID: plan.ID, ExcludedSubagentIds: plan.ExcludedSubagentIDs}, diags)
		}},
		{"excluded_connector_ids", plan.ExcludedConnectorIDs, prior.ExcludedConnectorIDs, func() bool {
			helper := AgentKnowledgeSourceExclusionsResource{client: r.client}
			return helper.replaceExclusions(ctx, &AgentKnowledgeSourceExclusionsResourceModel{AgentID: plan.ID, ExcludedConnectorIds: plan.ExcludedConnectorIDs}, diags)
		}},
	}
	for _, change := range changes {
		if !agentConfigured(ctx, config, change.name, diags) || change.desired.Equal(change.before) {
			continue
		}
		if !change.write() {
			return
		}
	}
}
func (r *AgentResource) writeInlineSkillPolicy(ctx context.Context, plan, prior *AgentResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	if diags.HasError() || !agentConfigured(ctx, config, "skills", diags) || plan.SkillPolicy.Equal(prior.SkillPolicy) {
		return
	}
	var inline agentInlinePolicy
	diags.Append(plan.SkillPolicy.As(ctx, &inline, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	helper := AgentSkillPolicyResource{client: r.client}
	current := helper.get(ctx, plan.ID.ValueString(), diags)
	if current == nil {
		if !diags.HasError() {
			diags.AddError("Agent not found", "The skill policy's agent no longer exists.")
		}
		return
	}
	if !prior.ActivationSkillPolicyRevision.IsNull() && !prior.ActivationSkillPolicyRevision.IsUnknown() && current.Revision != prior.ActivationSkillPolicyRevision.ValueInt64() {
		diags.AddError("Concurrent Skill Policy Update", "The policy changed after refresh; refresh and review a new plan.")
		return
	}
	target := AgentSkillPolicyResourceModel{AgentID: plan.ID, Mode: inline.Mode, AllowedReferences: inline.Allowed, ExcludedReferences: inline.Excluded}
	helper.replace(ctx, &target, current, diags)
}
func validateInlineSkillPolicy(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	var policy types.Object
	diags.Append(config.GetAttribute(ctx, path.Root("skills"), &policy)...)
	if diags.HasError() || policy.IsNull() || policy.IsUnknown() {
		return
	}
	var inline agentInlinePolicy
	diags.Append(policy.As(ctx, &inline, basetypes.ObjectAsOptions{UnhandledUnknownAsEmpty: true})...)
	for name, set := range map[string]types.Set{"allowed": inline.Allowed, "excluded": inline.Excluded} {
		if set.IsNull() || set.IsUnknown() {
			continue
		}
		var refs []skillReferenceModel
		diags.Append(set.ElementsAs(ctx, &refs, true)...)
		for _, ref := range refs {
			if ref.Source.IsUnknown() || ref.SkillID.IsUnknown() || ref.McpServerID.IsUnknown() || ref.URI.IsUnknown() || ref.PluginID.IsUnknown() || ref.SkillPath.IsUnknown() {
				continue
			}
			wire := ref.wire()
			valid := false
			switch wire.Source {
			case "native":
				valid = validSkillUUID(wire.SkillID) && wire.McpServerID == "" && wire.URI == "" && wire.PluginID == "" && wire.SkillPath == ""
			case "external_mcp":
				valid = validSkillUUID(wire.McpServerID) && wire.URI != "" && wire.SkillID == "" && wire.PluginID == "" && wire.SkillPath == ""
			case "plugin":
				valid = validSkillUUID(wire.PluginID) && wire.SkillPath != "" && wire.SkillID == "" && wire.McpServerID == "" && wire.URI == ""
			}
			if !valid {
				diags.AddAttributeError(path.Root("skills").AtName(name), "Invalid skill reference", "Use skill_id for native, mcp_server_id plus uri for external_mcp, or plugin_id plus skill_path for plugin references.")
			}
		}
	}
}

func nullUnknownAgentRelationships(data *AgentResourceModel) {
	for _, value := range []*types.Set{&data.SubagentIDs, &data.RemoteAgentIDs, &data.ExcludedSubagentIDs, &data.ExcludedConnectorIDs, &data.ExcludedToolIDs} {
		if value.IsUnknown() {
			*value = types.SetNull(types.StringType)
		}
	}
	if data.Hooks.IsUnknown() {
		data.Hooks = types.SetNull(agentHookType)
	}
	if data.Tools.IsUnknown() {
		data.Tools = types.SetNull(agentInlineToolType)
	}
	if data.SkillPolicy.IsUnknown() {
		data.SkillPolicy = types.ObjectNull(agentInlinePolicyType.AttrTypes)
	}
	if data.SkillPolicyHiddenAllowedCount.IsUnknown() {
		data.SkillPolicyHiddenAllowedCount = types.Int64Null()
	}
	if data.SkillPolicyHiddenExcludedCount.IsUnknown() {
		data.SkillPolicyHiddenExcludedCount = types.Int64Null()
	}
}

func (r *AgentResource) readRelationships(ctx context.Context, data *AgentResourceModel, diags *diag.Diagnostics) {
	r.readAssignments(ctx, data, diags)
	if !diags.HasError() {
		r.readInlineSkillPolicy(ctx, data, diags)
	}
}
func (r *AgentResource) writeRelationships(ctx context.Context, plan, prior *AgentResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	r.writeAssignments(ctx, plan, prior, config, diags)
	if !diags.HasError() {
		r.writeInlineSkillPolicy(ctx, plan, prior, config, diags)
	}
}
