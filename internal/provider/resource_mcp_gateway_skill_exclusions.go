package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var (
	_ resource.Resource                = &MCPGatewaySkillExclusionsResource{}
	_ resource.ResourceWithImportState = &MCPGatewaySkillExclusionsResource{}
)

func NewMCPGatewaySkillExclusionsResource() resource.Resource {
	return &MCPGatewaySkillExclusionsResource{}
}

type MCPGatewaySkillExclusionsResource struct {
	client *client.ClientWithResponses
}

// MCPGatewaySkillExclusionsResourceModel models the FULL Auto-skill-mode exclusion
// list of one agent. The backend endpoint is a full replace, so the resource
// is authoritative: at most one per agent, and out-of-band additions are
// reverted on the next apply.
type MCPGatewaySkillExclusionsResourceModel struct {
	ID               types.String `tfsdk:"id"`
	GatewayID        types.String `tfsdk:"gateway_id"`
	ExcludedSkillIds types.Set    `tfsdk:"excluded_skill_ids"`
}

func (r *MCPGatewaySkillExclusionsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_gateway_skill_exclusions"
}

func (r *MCPGatewaySkillExclusionsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Authoritative exclusions from one MCP gateway's Auto skill publication. Exclusions persist while Auto mode is disabled. Declare one per gateway; destroy clears exclusions while preserving the gateway and its publication policy.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `gateway_id` — the exclusion list has no id of its own",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"gateway_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "MCP gateway UUID from archestra_mcp_gateway.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"excluded_skill_ids": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Skill UUIDs excluded from Auto publication; an empty set clears all exclusions.",
			},
		},
	}
}

func (r *MCPGatewaySkillExclusionsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *MCPGatewaySkillExclusionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MCPGatewaySkillExclusionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.replaceExclusions(ctx, &plan, &resp.Diagnostics) {
		return
	}
	plan.ID = plan.GatewayID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MCPGatewaySkillExclusionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MCPGatewaySkillExclusionsResourceModel
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
	gatewayUUID, err := uuid.Parse(data.GatewayID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.GetAgentSkillExclusionsWithResponse(ctx, gatewayUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent gateway skill exclusions: %s", err))
		return
	}
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	ids := make([]string, len(apiResp.JSON200.ExcludedSkillIds))
	for i, id := range apiResp.JSON200.ExcludedSkillIds {
		ids[i] = id.String()
	}
	set, d := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(d...)
	data.ExcludedSkillIds = set
	data.ID = data.GatewayID

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPGatewaySkillExclusionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MCPGatewaySkillExclusionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.replaceExclusions(ctx, &plan, &resp.Diagnostics) {
		return
	}
	plan.ID = plan.GatewayID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete clears the agent's exclusion list — the resource owns the whole set,
// so destroy means "no exclusions", not "stop managing".
func (r *MCPGatewaySkillExclusionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MCPGatewaySkillExclusionsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gatewayUUID, err := uuid.Parse(data.GatewayID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.UpdateAgentSkillExclusionsWithResponse(ctx, gatewayUUID,
		client.UpdateAgentSkillExclusionsJSONRequestBody{ExcludedSkillIds: []openapi_types.UUID{}})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to clear agent gateway skill exclusions: %s", err))
		return
	}
	// Agent already gone → nothing to clear.
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

// ImportState imports by agent UUID (the list has no id of its own).
func (r *MCPGatewaySkillExclusionsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("gateway_id"), req.ID)...)
}

// AttrSpecs implements resourceWithAttrSpec (see specdrift_test.go). Both
// fields ride a typed body / the URL path rather than a merge patch, but the
// declaration keeps the schema↔spec drift lint active.
func (r *MCPGatewaySkillExclusionsResource) AttrSpecs() []AttrSpec {
	return mcpGatewaySkillExclusionsAttrSpec
}

func (r *MCPGatewaySkillExclusionsResource) APIShape() any {
	return client.GetAgentSkillExclusionsResponse{}
}

func (r *MCPGatewaySkillExclusionsResource) KnownIntentionallySkipped() []string {
	// Hydrated skill summaries duplicate excluded_skill_ids.
	return []string{"skills"}
}

var mcpGatewaySkillExclusionsAttrSpec = []AttrSpec{
	{TFName: "gateway_id", Kind: Synthetic},
	{TFName: "excluded_skill_ids", JSONName: "excludedSkillIds", Kind: Set},
}

// replaceExclusions PUTs the full plan-side exclusion set.
func (r *MCPGatewaySkillExclusionsResource) replaceExclusions(ctx context.Context, plan *MCPGatewaySkillExclusionsResourceModel, diags *diag.Diagnostics) bool {
	gatewayUUID, err := uuid.Parse(plan.GatewayID.ValueString())
	if err != nil {
		diags.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return false
	}

	var rawIds []string
	diags.Append(plan.ExcludedSkillIds.ElementsAs(ctx, &rawIds, false)...)
	if diags.HasError() {
		return false
	}
	skillIds := make([]openapi_types.UUID, len(rawIds))
	for i, raw := range rawIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			diags.AddError("Invalid Skill ID", fmt.Sprintf("Unable to parse skill ID %q: %s", raw, err))
			return false
		}
		skillIds[i] = id
	}

	apiResp, err := r.client.UpdateAgentSkillExclusionsWithResponse(ctx, gatewayUUID,
		client.UpdateAgentSkillExclusionsJSONRequestBody{ExcludedSkillIds: skillIds})
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to replace agent gateway skill exclusions: %s", err))
		return false
	}
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return false
	}
	confirmed := make([]string, 0, len(apiResp.JSON200.ExcludedSkillIds))
	for _, id := range apiResp.JSON200.ExcludedSkillIds {
		confirmed = append(confirmed, id.String())
	}
	if !agentSet(ctx, confirmed, diags).Equal(plan.ExcludedSkillIds) {
		diags.AddError("Incomplete exclusion replacement", "The API did not retain the complete requested exclusion set. Refresh and verify access before retrying.")
		return false
	}

	return true
}
