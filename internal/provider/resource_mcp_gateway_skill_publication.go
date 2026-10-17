package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &MCPGatewaySkillPublicationResource{}

func NewMCPGatewaySkillPublicationResource() resource.Resource {
	return &MCPGatewaySkillPublicationResource{}
}

type MCPGatewaySkillPublicationResource struct{ client *client.ClientWithResponses }
type MCPGatewaySkillPublicationResourceModel struct {
	ID              types.String `tfsdk:"id"`
	GatewayID       types.String `tfsdk:"gateway_id"`
	AccessAllSkills types.Bool   `tfsdk:"access_all_skills"`
	SkillIDs        types.Set    `tfsdk:"skill_ids"`
}

func (r *MCPGatewaySkillPublicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_gateway_skill_publication"
}
func (r *MCPGatewaySkillPublicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Authoritative skills published by one MCP gateway. Publication delegates content access to the gateway's callers. Declare one per gateway. Auto mode and explicit assignments persist independently; exclusions use archestra_mcp_gateway_skill_exclusions. Destroy disables Auto mode and clears explicit assignments, preserving the gateway and exclusions.", Attributes: map[string]schema.Attribute{
		"id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Gateway UUID identifying the publication policy."},
		"gateway_id":        schema.StringAttribute{Required: true, MarkdownDescription: "MCP gateway UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"access_all_skills": schema.BoolAttribute{Required: true, MarkdownDescription: "Whether to publish all eligible organization skills automatically."},
		"skill_ids":         schema.SetAttribute{Required: true, ElementType: types.StringType, Validators: []validator.Set{setvalidator.SizeAtMost(1000)}, MarkdownDescription: "Complete explicit skill assignments, retained while Auto mode is enabled."},
	}}
}
func (r *MCPGatewaySkillPublicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	r.client = c
}
func (r *MCPGatewaySkillPublicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MCPGatewaySkillPublicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.replace(ctx, &plan, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *MCPGatewaySkillPublicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MCPGatewaySkillPublicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	if !readSkillPublicationGateway(ctx, r.client, data.GatewayID.ValueString(), &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	id, err := uuid.Parse(data.GatewayID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Gateway ID", err.Error())
		return
	}
	result, err := r.client.GetAgentSkillsWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read gateway skill publication: %s", err))
		return
	}
	if result.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read gateway skill publication: status %d", result.StatusCode()))
		return
	}
	data.ID = data.GatewayID
	data.AccessAllSkills = types.BoolValue(result.JSON200.AccessAllSkills)
	ids := make([]string, 0, len(result.JSON200.SkillIds))
	for _, id := range result.JSON200.SkillIds {
		ids = append(ids, id.String())
	}
	set, d := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(d...)
	data.SkillIDs = set
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *MCPGatewaySkillPublicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MCPGatewaySkillPublicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.replace(ctx, &plan, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *MCPGatewaySkillPublicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MCPGatewaySkillPublicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.GatewayID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Gateway ID", err.Error())
		return
	}
	result, err := r.client.UpdateAgentSkillsWithResponse(ctx, id, client.UpdateAgentSkillsJSONRequestBody{AccessAllSkills: false, SkillIds: []uuid.UUID{}})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to clear gateway skill publication: %s", err))
		return
	}
	if result.JSON200 == nil && result.StatusCode() != 404 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to clear gateway skill publication: status %d", result.StatusCode()))
	}
}
func (r *MCPGatewaySkillPublicationResource) replace(ctx context.Context, plan *MCPGatewaySkillPublicationResourceModel, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(plan.GatewayID.ValueString())
	if err != nil {
		diags.AddError("Invalid Gateway ID", err.Error())
		return false
	}
	var raw []string
	diags.Append(plan.SkillIDs.ElementsAs(ctx, &raw, false)...)
	if diags.HasError() {
		return false
	}
	ids := make([]uuid.UUID, 0, len(raw))
	for _, value := range raw {
		id, err := uuid.Parse(value)
		if err != nil {
			diags.AddError("Invalid Skill ID", err.Error())
			return false
		}
		ids = append(ids, id)
	}
	result, err := r.client.UpdateAgentSkillsWithResponse(ctx, id, client.UpdateAgentSkillsJSONRequestBody{AccessAllSkills: plan.AccessAllSkills.ValueBool(), SkillIds: ids})
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to replace gateway skill publication: %s", err))
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to replace gateway skill publication: status %d", result.StatusCode()))
		return false
	}
	actual := make([]string, 0, len(result.JSON200.SkillIds))
	for _, id := range result.JSON200.SkillIds {
		actual = append(actual, id.String())
	}
	set, d := types.SetValueFrom(ctx, types.StringType, actual)
	diags.Append(d...)
	if !set.Equal(plan.SkillIDs) || result.JSON200.AccessAllSkills != plan.AccessAllSkills.ValueBool() {
		diags.AddError("Incomplete Skill Publication", "The API did not persist the complete requested publication policy.")
		return false
	}
	plan.ID = plan.GatewayID
	return !diags.HasError()
}
func (r *MCPGatewaySkillPublicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := uuid.Parse(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid Gateway ID", "Import by MCP gateway UUID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("gateway_id"), req.ID)...)
}
func (r *MCPGatewaySkillPublicationResource) AttrSpecs() []AttrSpec {
	return []AttrSpec{{TFName: "gateway_id", Kind: Synthetic}, {TFName: "access_all_skills", JSONName: "accessAllSkills", Kind: Scalar}, {TFName: "skill_ids", JSONName: "skillIds", Kind: Set}}
}
func (r *MCPGatewaySkillPublicationResource) APIShape() any { return client.GetAgentSkillsResponse{} }
func (r *MCPGatewaySkillPublicationResource) KnownIntentionallySkipped() []string {
	// Hydrated display summaries duplicate the stable skill_ids set.
	return []string{"skills"}
}
