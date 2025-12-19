package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ProfileToolResource{}
var _ resource.ResourceWithImportState = &ProfileToolResource{}

func NewProfileToolResource() resource.Resource {
	return &ProfileToolResource{}
}

// ProfileToolResource defines the resource implementation.
type ProfileToolResource struct {
	client *client.ClientWithResponses
}

// ProfileToolResourceModel describes the resource data model.
type ProfileToolResourceModel struct {
	ID                                   types.String `tfsdk:"id"`
	ProfileID                            types.String `tfsdk:"profile_id"`
	ToolID                               types.String `tfsdk:"tool_id"`
	CredentialSourceMCPServerID          types.String `tfsdk:"credential_source_mcp_server_id"`
	ExecutionSourceMCPServerID           types.String `tfsdk:"execution_source_mcp_server_id"`
	UseDynamicTeamCredential             types.Bool   `tfsdk:"use_dynamic_team_credential"`
	AllowUsageWhenUntrustedDataIsPresent types.Bool   `tfsdk:"allow_usage_when_untrusted_data_is_present"`
	ToolResultTreatment                  types.String `tfsdk:"tool_result_treatment"`
	ResponseModifierTemplate             types.String `tfsdk:"response_modifier_template"`
}

func (r *ProfileToolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile_tool"
}

func (r *ProfileToolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns a tool to an Archestra Profile and configures its execution and security policies.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the profile-tool assignment (Composite ID: profile_id:tool_id)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the Profile to assign the tool to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tool_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the Tool to assign",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential_source_mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the MCP Server instance to use for credentials/authentication",
				Optional:            true,
			},
			"execution_source_mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the MCP Server instance to use for execution",
				Optional:            true,
			},
			"use_dynamic_team_credential": schema.BoolAttribute{
				MarkdownDescription: "If true, dynamically resolves credentials based on the team context at runtime",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"allow_usage_when_untrusted_data_is_present": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow tool usage when untrusted data is present",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"tool_result_treatment": schema.StringAttribute{
				MarkdownDescription: "How to treat tool results (trusted, sanitize_with_dual_llm, untrusted)",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"response_modifier_template": schema.StringAttribute{
				MarkdownDescription: "Template string to modify the tool response before it reaches the model",
				Optional:            true,
			},
		},
	}
}

func (r *ProfileToolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *ProfileToolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	profileIDStr := data.ProfileID.ValueString()
	toolIDStr := data.ToolID.ValueString()

	profileUUID, err := uuid.Parse(profileIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(toolIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	// Prepare request body
	body := client.AssignToolToAgentJSONRequestBody{}

	var credentialSourceID *uuid.UUID
	if !data.CredentialSourceMCPServerID.IsNull() && !data.CredentialSourceMCPServerID.IsUnknown() {
		id, err := uuid.Parse(data.CredentialSourceMCPServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Credential Source MCP Server ID", fmt.Sprintf("Unable to parse ID: %s", err))
			return
		}
		credentialSourceID = &id
		body.CredentialSourceMcpServerId = credentialSourceID
	}

	var executionSourceID *uuid.UUID
	if !data.ExecutionSourceMCPServerID.IsNull() && !data.ExecutionSourceMCPServerID.IsUnknown() {
		id, err := uuid.Parse(data.ExecutionSourceMCPServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Execution Source MCP Server ID", fmt.Sprintf("Unable to parse ID: %s", err))
			return
		}
		executionSourceID = &id
		body.ExecutionSourceMcpServerId = executionSourceID
	}

	if !data.UseDynamicTeamCredential.IsNull() && !data.UseDynamicTeamCredential.IsUnknown() {
		val := data.UseDynamicTeamCredential.ValueBool()
		body.UseDynamicTeamCredential = &val
	}

	assignResp, err := r.client.AssignToolToAgentWithResponse(ctx, profileUUID, toolUUID, body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to assign tool to profile, got error: %s", err))
		return
	}

	if assignResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("AssignToolToAgent: Expected 200 OK, got status %d", assignResp.StatusCode()),
		)
		return
	}

	needsUpdate := false
	updateBody := client.UpdateAgentToolJSONRequestBody{}

	if !data.AllowUsageWhenUntrustedDataIsPresent.IsNull() {
		val := data.AllowUsageWhenUntrustedDataIsPresent.ValueBool()
		updateBody.AllowUsageWhenUntrustedDataIsPresent = &val
		needsUpdate = true
	}

	if !data.ToolResultTreatment.IsNull() {
		val := client.UpdateAgentToolJSONBodyToolResultTreatment(data.ToolResultTreatment.ValueString())
		updateBody.ToolResultTreatment = &val
		needsUpdate = true
	}

	if !data.ResponseModifierTemplate.IsNull() {
		val := data.ResponseModifierTemplate.ValueString()
		updateBody.ResponseModifierTemplate = &val
		needsUpdate = true
	}

	if credentialSourceID != nil {
		updateBody.CredentialSourceMcpServerId = credentialSourceID
	}
	if executionSourceID != nil {
		updateBody.ExecutionSourceMcpServerId = executionSourceID
	}
	if !data.UseDynamicTeamCredential.IsNull() && !data.UseDynamicTeamCredential.IsUnknown() {
		val := data.UseDynamicTeamCredential.ValueBool()
		updateBody.UseDynamicTeamCredential = &val
	}

	profileToolID, err := r.findProfileToolID(ctx, profileUUID, toolUUID)
	if err != nil {
		resp.Diagnostics.AddError("Lookup Error", fmt.Sprintf("Unable to find assigned tool: %s", err))
		return
	}

	if needsUpdate {
		updateResp, err := r.client.UpdateAgentToolWithResponse(ctx, profileToolID, updateBody)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update tool configuration, got error: %s", err))
			return
		}
		if updateResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("UpdateAgentTool: Expected 200 OK, got status %d", updateResp.StatusCode()),
			)
			return
		}
	}

	r.readState(ctx, profileToolID, profileUUID, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%s:%s", profileIDStr, toolIDStr))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Parse composite ID
	parts := strings.Split(data.ID.ValueString(), ":")
	if len(parts) != 2 {
		resp.State.RemoveResource(ctx) // ID format changed or invalid
		return
	}

	profileIDStr := parts[0]
	toolIDStr := parts[1]

	profileUUID, err := uuid.Parse(profileIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(toolIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	profileToolID, err := r.findProfileToolID(ctx, profileUUID, toolUUID)
	if err != nil {
		// If not found, remove from state
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to find profile tool: %s", err))
		return
	}

	r.readState(ctx, profileToolID, profileUUID, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	profileIDStr := data.ProfileID.ValueString()
	toolIDStr := data.ToolID.ValueString()

	profileUUID, err := uuid.Parse(profileIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(toolIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	profileToolID, err := r.findProfileToolID(ctx, profileUUID, toolUUID)
	if err != nil {
		resp.Diagnostics.AddError("Lookup Error", fmt.Sprintf("Unable to find assigned tool: %s", err))
		return
	}

	updateBody := client.UpdateAgentToolJSONRequestBody{}

	// Update credentials/sources if changed
	if !data.CredentialSourceMCPServerID.IsNull() {
		id, err := uuid.Parse(data.CredentialSourceMCPServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Credential Source MCP Server ID", fmt.Sprintf("Unable to parse ID: %s", err))
			return
		}
		updateBody.CredentialSourceMcpServerId = &id
	}
	// If null, we do nothing. The struct field is (*UUID)(nil), which serializes to null in JSON
	// because the generated client logic (or lack of omitempty) handles it.

	if !data.ExecutionSourceMCPServerID.IsNull() {
		id, err := uuid.Parse(data.ExecutionSourceMCPServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Execution Source MCP Server ID", fmt.Sprintf("Unable to parse ID: %s", err))
			return
		}
		updateBody.ExecutionSourceMcpServerId = &id
	}

	if !data.UseDynamicTeamCredential.IsNull() {
		val := data.UseDynamicTeamCredential.ValueBool()
		updateBody.UseDynamicTeamCredential = &val
	} else {
		// Explicitly set to false if unset/null
		val := false
		updateBody.UseDynamicTeamCredential = &val
	}

	if !data.AllowUsageWhenUntrustedDataIsPresent.IsNull() {
		val := data.AllowUsageWhenUntrustedDataIsPresent.ValueBool()
		updateBody.AllowUsageWhenUntrustedDataIsPresent = &val
	}

	if !data.ToolResultTreatment.IsNull() {
		val := client.UpdateAgentToolJSONBodyToolResultTreatment(data.ToolResultTreatment.ValueString())
		updateBody.ToolResultTreatment = &val
	}

	if !data.ResponseModifierTemplate.IsNull() {
		val := data.ResponseModifierTemplate.ValueString()
		updateBody.ResponseModifierTemplate = &val
	}

	updateResp, err := r.client.UpdateAgentToolWithResponse(ctx, profileToolID, updateBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update profile tool, got error: %s", err))
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", updateResp.StatusCode()),
		)
		return
	}

	// Read state back
	r.readState(ctx, profileToolID, profileUUID, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	profileIDStr := data.ProfileID.ValueString()
	toolIDStr := data.ToolID.ValueString()

	profileUUID, err := uuid.Parse(profileIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(toolIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	delResp, err := r.client.UnassignToolFromAgentWithResponse(ctx, profileUUID, toolUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to unassign tool, got error: %s", err))
		return
	}

	if delResp.StatusCode() != 200 && delResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", delResp.StatusCode()),
		)
		return
	}
}

func (r *ProfileToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by composite ID: "profile_id:tool_id"
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper: Find the ProfileTool ID (which is the relationship ID, not the Tool ID).
func (r *ProfileToolResource) findProfileToolID(ctx context.Context, profileID, toolID uuid.UUID) (openapi_types.UUID, error) {
	// Helper logic to list tools and find the one matching toolID
	limit := 100
	params := &client.GetAllAgentToolsParams{
		AgentId: &profileID,
		Limit:   &limit,
	}

	// This finds based on ProfileID.
	resp, err := r.client.GetAllAgentToolsWithResponse(ctx, params)
	if err != nil {
		return uuid.Nil, err
	}
	if resp.JSON200 == nil {
		return uuid.Nil, fmt.Errorf("listing tools failed with status %d", resp.StatusCode())
	}

	for _, at := range resp.JSON200.Data {
		if at.Tool.Id == toolID.String() {
			return at.Id, nil
		}
	}

	return uuid.Nil, fmt.Errorf("tool assignment not found")
}

// Helper: Read state from API into model.
func (r *ProfileToolResource) readState(
	ctx context.Context,
	profileToolID openapi_types.UUID,
	profileUUID uuid.UUID,
	data *ProfileToolResourceModel,
	diags *diag.Diagnostics,
) {
	limit := 100
	// We need the ProfileID to filter efficiently.
	params := &client.GetAllAgentToolsParams{
		AgentId: &profileUUID,
		Limit:   &limit,
	}

	resp, err := r.client.GetAllAgentToolsWithResponse(ctx, params)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read profile tool: %s", err))
		return
	}

	if resp.JSON200 == nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read profile tool: unexpected status code %d", resp.StatusCode()))
		return
	}

	for i := range resp.JSON200.Data {
		at := &resp.JSON200.Data[i]
		if at.Id == profileToolID {
			// Map to model directly
			data.ProfileID = types.StringValue(at.Agent.Id)
			data.ToolID = types.StringValue(at.Tool.Id)
			data.AllowUsageWhenUntrustedDataIsPresent = types.BoolValue(at.AllowUsageWhenUntrustedDataIsPresent)
			data.ToolResultTreatment = types.StringValue(string(at.ToolResultTreatment))
			data.UseDynamicTeamCredential = types.BoolValue(at.UseDynamicTeamCredential)

			if at.CredentialSourceMcpServerId != nil {
				data.CredentialSourceMCPServerID = types.StringValue(at.CredentialSourceMcpServerId.String())
			} else {
				data.CredentialSourceMCPServerID = types.StringNull()
			}

			if at.ExecutionSourceMcpServerId != nil {
				data.ExecutionSourceMCPServerID = types.StringValue(at.ExecutionSourceMcpServerId.String())
			} else {
				data.ExecutionSourceMCPServerID = types.StringNull()
			}

			if at.ResponseModifierTemplate != nil && *at.ResponseModifierTemplate != "" {
				data.ResponseModifierTemplate = types.StringValue(*at.ResponseModifierTemplate)
			} else {
				data.ResponseModifierTemplate = types.StringNull()
			}
			return
		}
	}

	diags.AddError("Not Found", "Profile tool assignment not found")
}
