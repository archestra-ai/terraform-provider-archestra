package provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type gatewayInlineSkills struct {
	Mode        types.String `tfsdk:"mode"`
	IDs         types.Set    `tfsdk:"ids"`
	ExcludedIDs types.Set    `tfsdk:"excluded_ids"`
}

var gatewayInlineSkillsType = types.ObjectType{AttrTypes: map[string]attr.Type{"mode": types.StringType, "ids": types.SetType{ElemType: types.StringType}, "excluded_ids": types.SetType{ElemType: types.StringType}}}

func gatewayRelationshipAttributes() map[string]schema.Attribute {
	attributes := agentRelationshipAttributes()
	for _, name := range []string{"skills", "skill_policy_hidden_allowed_count", "skill_policy_hidden_excluded_count"} {
		delete(attributes, name)
	}
	ids := func(description string) schema.SetAttribute {
		return schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: description, Validators: []validator.Set{setvalidator.SizeAtMost(1000), setvalidator.ValueStringsAre(stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use lowercase canonical skill UUIDs."))}}
	}
	attributes["skills"] = schema.SingleNestedAttribute{Optional: true, Computed: true, MarkdownDescription: "Skills published through this gateway; omit to preserve existing publication settings.", PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, Attributes: map[string]schema.Attribute{
		"mode":         schema.StringAttribute{Required: true, MarkdownDescription: "Publish all eligible skills automatically, or use the explicit manual selection.", Validators: []validator.String{stringvalidator.OneOf("all", "manual")}},
		"ids":          ids("Complete explicit skill selection, retained independently of automatic mode."),
		"excluded_ids": ids("Skills excluded from automatic publication; an empty set clears exclusions."),
	}}
	attributes["tools"] = agentToolsAttribute()
	attributes["excluded_tool_ids"] = agentIDSetAttribute("Tools excluded from automatic tool access; omit to preserve or set an empty set to clear.", 0)
	return attributes
}
func gatewayRelationshipAttrSpecs() []AttrSpec {
	return []AttrSpec{{TFName: "subagent_ids", Kind: Synthetic}, {TFName: "remote_agent_ids", Kind: Synthetic}, {TFName: "excluded_subagent_ids", Kind: Synthetic}, {TFName: "excluded_connector_ids", Kind: Synthetic}, {TFName: "skills", Kind: Synthetic}, {TFName: "tools", Kind: Synthetic}, {TFName: "excluded_tool_ids", Kind: Synthetic}}
}
func gatewayAgentView(data *McpGatewayResourceModel) AgentResourceModel {
	return AgentResourceModel{ID: data.ID, SubagentIDs: data.SubagentIDs, RemoteAgentIDs: data.RemoteAgentIDs, ExcludedSubagentIDs: data.ExcludedSubagentIDs, ExcludedConnectorIDs: data.ExcludedConnectorIDs, Tools: data.Tools, ExcludedToolIDs: data.ExcludedToolIDs}
}
func copyGatewayRelations(data *McpGatewayResourceModel, view AgentResourceModel) {
	data.SubagentIDs = view.SubagentIDs
	data.RemoteAgentIDs = view.RemoteAgentIDs
	data.ExcludedSubagentIDs = view.ExcludedSubagentIDs
	data.ExcludedConnectorIDs = view.ExcludedConnectorIDs
	data.Tools = view.Tools
	data.ExcludedToolIDs = view.ExcludedToolIDs
}
func nullUnknownGatewayRelationships(data *McpGatewayResourceModel) {
	view := gatewayAgentView(data)
	nullUnknownAgentRelationships(&view)
	copyGatewayRelations(data, view)
	if data.Skills.IsUnknown() {
		data.Skills = types.ObjectNull(gatewayInlineSkillsType.AttrTypes)
	}
}
func (r *McpGatewayResource) readRelationships(ctx context.Context, data *McpGatewayResourceModel, diags *diag.Diagnostics) {
	manager := AgentResource{client: r.client}
	view := gatewayAgentView(data)
	manager.readAssignments(ctx, &view, diags)
	if diags.HasError() {
		return
	}
	manager.readTools(ctx, &view, diags)
	if diags.HasError() {
		return
	}
	copyGatewayRelations(data, view)
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid gateway ID", err.Error())
		return
	}
	publication, err := r.client.GetAgentSkillsWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read gateway skills", 0, err)
		return
	}
	if publication.JSON200 == nil {
		agentRelationshipError(diags, "Read gateway skills", publication.StatusCode(), nil)
		return
	}
	exclusions, err := r.client.GetAgentSkillExclusionsWithResponse(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read gateway skill exclusions", 0, err)
		return
	}
	if exclusions.JSON200 == nil {
		agentRelationshipError(diags, "Read gateway skill exclusions", exclusions.StatusCode(), nil)
		return
	}
	selected := make([]string, 0, len(publication.JSON200.SkillIds))
	for _, id := range publication.JSON200.SkillIds {
		selected = append(selected, id.String())
	}
	excluded := make([]string, 0, len(exclusions.JSON200.ExcludedSkillIds))
	for _, id := range exclusions.JSON200.ExcludedSkillIds {
		excluded = append(excluded, id.String())
	}
	mode := "manual"
	if publication.JSON200.AccessAllSkills {
		mode = "all"
	}
	skills := gatewayInlineSkills{Mode: types.StringValue(mode), IDs: agentSet(ctx, selected, diags), ExcludedIDs: agentSet(ctx, excluded, diags)}
	value, d := types.ObjectValueFrom(ctx, gatewayInlineSkillsType.AttrTypes, &skills)
	diags.Append(d...)
	data.Skills = value
}
func (r *McpGatewayResource) writeRelationships(ctx context.Context, plan, prior *McpGatewayResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	manager := AgentResource{client: r.client}
	desired, before := gatewayAgentView(plan), gatewayAgentView(prior)
	manager.writeAssignments(ctx, &desired, &before, config, diags)
	if diags.HasError() {
		return
	}
	manager.writeTools(ctx, &desired, &before, config, diags)
	if diags.HasError() || !agentConfigured(ctx, config, "skills", diags) || plan.Skills.Equal(prior.Skills) {
		return
	}
	var skills gatewayInlineSkills
	diags.Append(plan.Skills.As(ctx, &skills, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	publication := MCPGatewaySkillPublicationResource{client: r.client}
	target := MCPGatewaySkillPublicationResourceModel{GatewayID: plan.ID, AccessAllSkills: types.BoolValue(skills.Mode.ValueString() == "all"), SkillIDs: skills.IDs}
	if !publication.replace(ctx, &target, diags) {
		return
	}
	exclusions := MCPGatewaySkillExclusionsResource{client: r.client}
	exclusions.replaceExclusions(ctx, &MCPGatewaySkillExclusionsResourceModel{GatewayID: plan.ID, ExcludedSkillIds: skills.ExcludedIDs}, diags)
}
func (r *McpGatewayResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *McpGatewayResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateInlineTools(ctx, req.Config, &resp.Diagnostics)
}
