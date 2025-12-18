package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ProfileToolResource{}
var _ resource.ResourceWithImportState = &ProfileToolResource{}

func NewProfileToolResource() resource.Resource {
	return &ProfileToolResource{}
}

type ProfileToolResource struct {
	client *client.ClientWithResponses
}

type ProfileToolResourceModel struct {
	ID                                   types.String `tfsdk:"id"`
	ProfileId                            types.String `tfsdk:"profile_id"`
	ToolId                               types.String `tfsdk:"tool_id"`
	CredentialSourceMcpServerId          types.String `tfsdk:"credential_source_mcp_server_id"`
	ExecutionSourceMcpServerId           types.String `tfsdk:"execution_source_mcp_server_id"`
	UseDynamicTeamCredential             types.Bool   `tfsdk:"use_dynamic_team_credential"`
	AllowUsageWhenUntrustedDataIsPresent types.Bool   `tfsdk:"allow_usage_when_untrusted_data_is_present"`
	ToolResultTreatment                  types.String `tfsdk:"tool_result_treatment"`
}

func (r *ProfileToolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile_tool"
}

func (r *ProfileToolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns a tool to a profile (agent) in Archestra.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal profile tool assignment identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile_id": schema.StringAttribute{
				MarkdownDescription: "The profile (agent) ID to assign the tool to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tool_id": schema.StringAttribute{
				MarkdownDescription: "The tool ID from the tool catalog to assign",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential_source_mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "MCP server ID to use for credentials",
				Optional:            true,
			},
			"execution_source_mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "MCP server ID to use for execution",
				Optional:            true,
			},
			"use_dynamic_team_credential": schema.BoolAttribute{
				MarkdownDescription: "Whether to use dynamic team credentials",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"allow_usage_when_untrusted_data_is_present": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow tool usage when untrusted data is present in the context",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"tool_result_treatment": schema.StringAttribute{
				MarkdownDescription: "How to treat the tool result: trusted, untrusted, or sanitize_with_dual_llm",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("trusted"),
				Validators: []validator.String{
					stringvalidator.OneOf("trusted", "untrusted", "sanitize_with_dual_llm"),
				},
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

	profileId, err := uuid.Parse(data.ProfileId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolId, err := uuid.Parse(data.ToolId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	useDynamicTeamCredential := data.UseDynamicTeamCredential.ValueBool()
	requestBody := client.AssignToolToAgentJSONRequestBody{
		UseDynamicTeamCredential: &useDynamicTeamCredential,
	}

	if !data.CredentialSourceMcpServerId.IsNull() && !data.CredentialSourceMcpServerId.IsUnknown() {
		credId, err := uuid.Parse(data.CredentialSourceMcpServerId.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Credential Source MCP Server ID", fmt.Sprintf("Unable to parse credential source MCP server ID: %s", err))
			return
		}
		requestBody.CredentialSourceMcpServerId = &credId
	}

	if !data.ExecutionSourceMcpServerId.IsNull() && !data.ExecutionSourceMcpServerId.IsUnknown() {
		execId, err := uuid.Parse(data.ExecutionSourceMcpServerId.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Execution Source MCP Server ID", fmt.Sprintf("Unable to parse execution source MCP server ID: %s", err))
			return
		}
		requestBody.ExecutionSourceMcpServerId = &execId
	}

	apiResp, err := r.client.AssignToolToAgentWithResponse(ctx, profileId, toolId, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to assign tool to profile, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	internalId, err := r.findProfileToolId(ctx, profileId, toolId)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to find created profile tool: %s", err))
		return
	}

	data.ID = types.StringValue(internalId.String())

	updateBody := client.UpdateAgentToolJSONRequestBody{}

	allow := data.AllowUsageWhenUntrustedDataIsPresent.ValueBool()
	updateBody.AllowUsageWhenUntrustedDataIsPresent = &allow

	treatment := client.UpdateAgentToolJSONBodyToolResultTreatment(data.ToolResultTreatment.ValueString())
	updateBody.ToolResultTreatment = &treatment

	updateResp, err := r.client.UpdateAgentToolWithResponse(ctx, internalId, updateBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update profile tool settings, got error: %s", err))
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK on update, got status %d: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200.AllowUsageWhenUntrustedDataIsPresent != nil {
		data.AllowUsageWhenUntrustedDataIsPresent = types.BoolValue(*updateResp.JSON200.AllowUsageWhenUntrustedDataIsPresent)
	}
	data.ToolResultTreatment = types.StringValue(string(updateResp.JSON200.ToolResultTreatment))
	if updateResp.JSON200.UseDynamicTeamCredential != nil {
		data.UseDynamicTeamCredential = types.BoolValue(*updateResp.JSON200.UseDynamicTeamCredential)
	}
	if updateResp.JSON200.CredentialSourceMcpServerId != nil {
		data.CredentialSourceMcpServerId = types.StringValue(updateResp.JSON200.CredentialSourceMcpServerId.String())
	}
	if updateResp.JSON200.ExecutionSourceMcpServerId != nil {
		data.ExecutionSourceMcpServerId = types.StringValue(updateResp.JSON200.ExecutionSourceMcpServerId.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	profileId, err := uuid.Parse(data.ProfileId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	internalId, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse profile tool ID: %s", err))
		return
	}

	listResp, err := r.client.GetAgentToolsWithResponse(ctx, profileId)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read profile tools, got error: %s", err))
		return
	}

	if listResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if listResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", listResp.StatusCode()),
		)
		return
	}

	var found bool
	for _, tool := range *listResp.JSON200 {
		if tool.Id == internalId {
			found = true
			if tool.CatalogId != nil {
				data.ToolId = types.StringValue(tool.CatalogId.String())
			}
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	// Call UpdateAgentTool with empty body to retrieve current field values for drift detection
	// The GetAgentTools endpoint doesn't return detailed configuration fields
	updateResp, err := r.client.UpdateAgentToolWithResponse(ctx, internalId, client.UpdateAgentToolJSONRequestBody{})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read profile tool details, got error: %s", err))
		return
	}

	if updateResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200.AllowUsageWhenUntrustedDataIsPresent != nil {
		data.AllowUsageWhenUntrustedDataIsPresent = types.BoolValue(*updateResp.JSON200.AllowUsageWhenUntrustedDataIsPresent)
	}
	data.ToolResultTreatment = types.StringValue(string(updateResp.JSON200.ToolResultTreatment))
	if updateResp.JSON200.UseDynamicTeamCredential != nil {
		data.UseDynamicTeamCredential = types.BoolValue(*updateResp.JSON200.UseDynamicTeamCredential)
	}
	if updateResp.JSON200.CredentialSourceMcpServerId != nil {
		data.CredentialSourceMcpServerId = types.StringValue(updateResp.JSON200.CredentialSourceMcpServerId.String())
	} else {
		data.CredentialSourceMcpServerId = types.StringNull()
	}
	if updateResp.JSON200.ExecutionSourceMcpServerId != nil {
		data.ExecutionSourceMcpServerId = types.StringValue(updateResp.JSON200.ExecutionSourceMcpServerId.String())
	} else {
		data.ExecutionSourceMcpServerId = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	internalId, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse profile tool ID: %s", err))
		return
	}

	updateBody := client.UpdateAgentToolJSONRequestBody{}

	if !data.CredentialSourceMcpServerId.IsNull() && !data.CredentialSourceMcpServerId.IsUnknown() {
		credId, err := uuid.Parse(data.CredentialSourceMcpServerId.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Credential Source MCP Server ID", fmt.Sprintf("Unable to parse credential source MCP server ID: %s", err))
			return
		}
		updateBody.CredentialSourceMcpServerId = &credId
	}

	if !data.ExecutionSourceMcpServerId.IsNull() && !data.ExecutionSourceMcpServerId.IsUnknown() {
		execId, err := uuid.Parse(data.ExecutionSourceMcpServerId.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Execution Source MCP Server ID", fmt.Sprintf("Unable to parse execution source MCP server ID: %s", err))
			return
		}
		updateBody.ExecutionSourceMcpServerId = &execId
	}

	useDynamic := data.UseDynamicTeamCredential.ValueBool()
	updateBody.UseDynamicTeamCredential = &useDynamic

	allow := data.AllowUsageWhenUntrustedDataIsPresent.ValueBool()
	updateBody.AllowUsageWhenUntrustedDataIsPresent = &allow

	treatment := client.UpdateAgentToolJSONBodyToolResultTreatment(data.ToolResultTreatment.ValueString())
	updateBody.ToolResultTreatment = &treatment

	apiResp, err := r.client.UpdateAgentToolWithResponse(ctx, internalId, updateBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update profile tool, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200.AllowUsageWhenUntrustedDataIsPresent != nil {
		data.AllowUsageWhenUntrustedDataIsPresent = types.BoolValue(*apiResp.JSON200.AllowUsageWhenUntrustedDataIsPresent)
	}
	data.ToolResultTreatment = types.StringValue(string(apiResp.JSON200.ToolResultTreatment))
	if apiResp.JSON200.UseDynamicTeamCredential != nil {
		data.UseDynamicTeamCredential = types.BoolValue(*apiResp.JSON200.UseDynamicTeamCredential)
	}
	if apiResp.JSON200.CredentialSourceMcpServerId != nil {
		data.CredentialSourceMcpServerId = types.StringValue(apiResp.JSON200.CredentialSourceMcpServerId.String())
	}
	if apiResp.JSON200.ExecutionSourceMcpServerId != nil {
		data.ExecutionSourceMcpServerId = types.StringValue(apiResp.JSON200.ExecutionSourceMcpServerId.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	profileId, err := uuid.Parse(data.ProfileId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolId, err := uuid.Parse(data.ToolId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	apiResp, err := r.client.UnassignToolFromAgentWithResponse(ctx, profileId, toolId)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to unassign tool from profile, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *ProfileToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format: profile_id/tool_id",
		)
		return
	}

	profileId, err := uuid.Parse(parts[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID from import: %s", err))
		return
	}

	toolId, err := uuid.Parse(parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID from import: %s", err))
		return
	}

	internalId, err := r.findProfileToolId(ctx, profileId, toolId)
	if err != nil {
		resp.Diagnostics.AddError("Import Error", fmt.Sprintf("Unable to find profile tool: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), internalId.String())...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("profile_id"), profileId.String())...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tool_id"), toolId.String())...)
}

func (r *ProfileToolResource) findProfileToolId(ctx context.Context, profileId, toolId uuid.UUID) (uuid.UUID, error) {
	apiResp, err := r.client.GetAgentToolsWithResponse(ctx, profileId)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("unable to list profile tools: %w", err)
	}

	if apiResp.JSON200 == nil {
		return uuid.UUID{}, fmt.Errorf("unexpected API response: status %d", apiResp.StatusCode())
	}

	for _, tool := range *apiResp.JSON200 {
		if tool.CatalogId != nil && *tool.CatalogId == toolId {
			return tool.Id, nil
		}
	}

	return uuid.UUID{}, fmt.Errorf("tool %s not found in profile %s", toolId, profileId)
}
