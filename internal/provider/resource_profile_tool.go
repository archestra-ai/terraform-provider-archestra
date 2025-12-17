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

var _ resource.Resource = &ProfileToolResource{}
var _ resource.ResourceWithImportState = &ProfileToolResource{}

func NewProfileToolResource() resource.Resource {
	return &ProfileToolResource{}
}

type ProfileToolResource struct {
	client *client.ClientWithResponses
}

type ProfileToolResourceModel struct {
	ID                           types.String `tfsdk:"id"`
	ProfileID                    types.String `tfsdk:"profile_id"`
	ToolID                       types.String `tfsdk:"tool_id"`
	CredentialSourceMcpServerID  types.String `tfsdk:"credential_source_mcp_server_id"`
	ExecutionSourceMcpServerID   types.String `tfsdk:"execution_source_mcp_server_id"`
	UseDynamicTeamCredential     types.Bool   `tfsdk:"use_dynamic_team_credential"`
	ResponseModifierTemplate     types.String `tfsdk:"response_modifier_template"`
	AllowUsageWhenUntrustedData  types.Bool   `tfsdk:"allow_usage_when_untrusted_data_is_present"`
	ToolResultTreatment          types.String `tfsdk:"tool_result_treatment"`
}

func (r *ProfileToolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile_tool"
}

func (r *ProfileToolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns an MCP tool to an Archestra profile (agent). " +
			"This resource manages the relationship between a profile and a tool, including credential sources and execution settings.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Agent tool identifier (internal ID from API)",
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
				MarkdownDescription: "The tool ID to assign (from archestra_mcp_server_tool data source)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential_source_mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "The MCP server ID to use as the credential source. If not specified, defaults to the tool's MCP server.",
				Optional:            true,
				Computed:            true,
			},
			"execution_source_mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "The MCP server ID to use for tool execution. If not specified, defaults to the tool's MCP server.",
				Optional:            true,
				Computed:            true,
			},
			"use_dynamic_team_credential": schema.BoolAttribute{
				MarkdownDescription: "Whether to use dynamic team credentials. Defaults to false.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"response_modifier_template": schema.StringAttribute{
				MarkdownDescription: "Optional template to modify tool responses",
				Optional:            true,
			},
			"allow_usage_when_untrusted_data_is_present": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow tool usage when untrusted data is present. Defaults to true.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"tool_result_treatment": schema.StringAttribute{
				MarkdownDescription: "How to treat tool results: 'trusted', 'untrusted', or 'sanitize_with_dual_llm'. Defaults to 'trusted'.",
				Optional:            true,
				Computed:            true,
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
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
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

	// Parse UUIDs
	profileID, err := uuid.Parse(data.ProfileID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolID, err := uuid.Parse(data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	// Build request body for assignment
	requestBody := client.AssignToolToAgentJSONRequestBody{}

	// Set credential source MCP server if provided
	if !data.CredentialSourceMcpServerID.IsNull() && !data.CredentialSourceMcpServerID.IsUnknown() {
		credSourceID, err := uuid.Parse(data.CredentialSourceMcpServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Credential Source MCP Server ID", fmt.Sprintf("Unable to parse: %s", err))
			return
		}
		credSourceUUID := openapi_types.UUID(credSourceID)
		requestBody.CredentialSourceMcpServerId = &credSourceUUID
	}

	// Set execution source MCP server if provided
	if !data.ExecutionSourceMcpServerID.IsNull() && !data.ExecutionSourceMcpServerID.IsUnknown() {
		execSourceID, err := uuid.Parse(data.ExecutionSourceMcpServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Execution Source MCP Server ID", fmt.Sprintf("Unable to parse: %s", err))
			return
		}
		execSourceUUID := openapi_types.UUID(execSourceID)
		requestBody.ExecutionSourceMcpServerId = &execSourceUUID
	}

	// Set use dynamic team credential if provided
	if !data.UseDynamicTeamCredential.IsNull() && !data.UseDynamicTeamCredential.IsUnknown() {
		useDynamic := data.UseDynamicTeamCredential.ValueBool()
		requestBody.UseDynamicTeamCredential = &useDynamic
	}

	// Assign tool to agent
	apiResp, err := r.client.AssignToolToAgentWithResponse(ctx, openapi_types.UUID(profileID), openapi_types.UUID(toolID), requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to assign tool to profile: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		if apiResp.JSON400 != nil {
			resp.Diagnostics.AddError(
				"API Error",
				fmt.Sprintf("Failed to assign tool: %s", apiResp.JSON400.Error.Message),
			)
		} else {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
			)
		}
		return
	}

	// Now we need to get the agent tool ID and potentially update additional settings
	// The assign endpoint just creates the relationship, but we may need to update other fields
	agentToolID, err := r.findAgentToolID(ctx, profileID, data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to find created agent tool: %s", err))
		return
	}

	data.ID = types.StringValue(agentToolID)

	// If we have additional settings to apply, update them now
	if r.needsUpdate(&data) {
		if err := r.updateAgentTool(ctx, agentToolID, &data); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update agent tool settings: %s", err))
			return
		}
	}

	// Read back the current state to get computed values
	r.readAgentTool(ctx, agentToolID, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentToolID := data.ID.ValueString()

	// Read the agent tool
	r.readAgentTool(ctx, agentToolID, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		// Check if it's a not found error
		if strings.Contains(resp.Diagnostics.Errors()[0].Summary(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}
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

	agentToolID := data.ID.ValueString()

	// Update the agent tool settings
	if err := r.updateAgentTool(ctx, agentToolID, &data); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update agent tool: %s", err))
		return
	}

	// Read back the updated state
	r.readAgentTool(ctx, agentToolID, &data, &resp.Diagnostics)
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

	// Parse UUIDs
	profileID, err := uuid.Parse(data.ProfileID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolID, err := uuid.Parse(data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	// Unassign tool from agent
	apiResp, err := r.client.UnassignToolFromAgentWithResponse(ctx, openapi_types.UUID(profileID), openapi_types.UUID(toolID))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to unassign tool from profile: %s", err))
		return
	}

	// Accept both 200 and 404 as success (tool already unassigned)
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *ProfileToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: profile_id:tool_id
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in format: profile_id:tool_id (both as UUIDs)",
		)
		return
	}

	profileID := parts[0]
	toolID := parts[1]

	// Validate UUIDs
	profileUUID, err := uuid.Parse(profileID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	if _, err := uuid.Parse(toolID); err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	// Find the agent tool ID
	agentToolID, err := r.findAgentToolID(ctx, profileUUID, toolID)
	if err != nil {
		resp.Diagnostics.AddError("Import Failed", fmt.Sprintf("Unable to find agent tool: %s", err))
		return
	}

	// Set the identifiers
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), agentToolID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("profile_id"), profileID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tool_id"), toolID)...)
}

// Helper functions

func (r *ProfileToolResource) findAgentToolID(ctx context.Context, profileID uuid.UUID, toolID string) (string, error) {
	limit := 100
	toolsResp, err := r.client.GetAllAgentToolsWithResponse(ctx, &client.GetAllAgentToolsParams{
		AgentId: (*openapi_types.UUID)(&profileID),
		Limit:   &limit,
	})
	if err != nil {
		return "", fmt.Errorf("unable to read agent tools: %w", err)
	}

	if toolsResp.JSON200 == nil {
		return "", fmt.Errorf("expected 200 OK, got status %d", toolsResp.StatusCode())
	}

	// Find the tool in the list
	for _, agentTool := range toolsResp.JSON200.Data {
		if agentTool.Tool.Id == toolID {
			return agentTool.Id.String(), nil
		}
	}

	return "", fmt.Errorf("agent tool not found for profile %s and tool %s", profileID, toolID)
}

func (r *ProfileToolResource) readAgentTool(ctx context.Context, agentToolID string, data *ProfileToolResourceModel, diags *diag.Diagnostics) {
	// Parse the agent tool ID
	agentToolUUID, err := uuid.Parse(agentToolID)
	if err != nil {
		diags.AddError("Invalid Agent Tool ID", fmt.Sprintf("Unable to parse agent tool ID: %s", err))
		return
	}

	// Get all agent tools and find ours by ID
	// We need to use GetAllAgentTools since there's no single GetAgentTool endpoint
	limit := 1000 // Large limit to ensure we find it
	toolsResp, err := r.client.GetAllAgentToolsWithResponse(ctx, &client.GetAllAgentToolsParams{
		Limit: &limit,
	})
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read agent tools: %s", err))
		return
	}

	if toolsResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", toolsResp.StatusCode()))
		return
	}

	// Find our specific agent tool
	foundIndex := -1
	for i := range toolsResp.JSON200.Data {
		if toolsResp.JSON200.Data[i].Id == agentToolUUID {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		diags.AddError("Not Found", fmt.Sprintf("Agent tool with ID %s not found", agentToolID))
		return
	}

	foundTool := toolsResp.JSON200.Data[foundIndex]

	// Map response to state
	data.ProfileID = types.StringValue(foundTool.Agent.Id)
	data.ToolID = types.StringValue(foundTool.Tool.Id)

	if foundTool.CredentialSourceMcpServerId != nil {
		data.CredentialSourceMcpServerID = types.StringValue(foundTool.CredentialSourceMcpServerId.String())
	} else {
		data.CredentialSourceMcpServerID = types.StringNull()
	}

	if foundTool.ExecutionSourceMcpServerId != nil {
		data.ExecutionSourceMcpServerID = types.StringValue(foundTool.ExecutionSourceMcpServerId.String())
	} else {
		data.ExecutionSourceMcpServerID = types.StringNull()
	}

	data.UseDynamicTeamCredential = types.BoolValue(foundTool.UseDynamicTeamCredential)
	data.AllowUsageWhenUntrustedData = types.BoolValue(foundTool.AllowUsageWhenUntrustedDataIsPresent)
	data.ToolResultTreatment = types.StringValue(string(foundTool.ToolResultTreatment))

	if foundTool.ResponseModifierTemplate != nil {
		data.ResponseModifierTemplate = types.StringValue(*foundTool.ResponseModifierTemplate)
	} else {
		data.ResponseModifierTemplate = types.StringNull()
	}
}

func (r *ProfileToolResource) needsUpdate(data *ProfileToolResourceModel) bool {
	// Check if any optional update fields are set
	return !data.ResponseModifierTemplate.IsNull() ||
		!data.AllowUsageWhenUntrustedData.IsNull() ||
		!data.ToolResultTreatment.IsNull()
}

func (r *ProfileToolResource) updateAgentTool(ctx context.Context, agentToolID string, data *ProfileToolResourceModel) error {
	agentToolUUID, err := uuid.Parse(agentToolID)
	if err != nil {
		return fmt.Errorf("unable to parse agent tool ID: %w", err)
	}

	// Build update request
	updateBody := client.UpdateAgentToolJSONRequestBody{}

	// Set credential source if provided
	if !data.CredentialSourceMcpServerID.IsNull() && !data.CredentialSourceMcpServerID.IsUnknown() {
		credSourceID, err := uuid.Parse(data.CredentialSourceMcpServerID.ValueString())
		if err != nil {
			return fmt.Errorf("unable to parse credential source MCP server ID: %w", err)
		}
		credSourceUUID := openapi_types.UUID(credSourceID)
		updateBody.CredentialSourceMcpServerId = &credSourceUUID
	}

	// Set execution source if provided
	if !data.ExecutionSourceMcpServerID.IsNull() && !data.ExecutionSourceMcpServerID.IsUnknown() {
		execSourceID, err := uuid.Parse(data.ExecutionSourceMcpServerID.ValueString())
		if err != nil {
			return fmt.Errorf("unable to parse execution source MCP server ID: %w", err)
		}
		execSourceUUID := openapi_types.UUID(execSourceID)
		updateBody.ExecutionSourceMcpServerId = &execSourceUUID
	}

	// Set use dynamic team credential if provided
	if !data.UseDynamicTeamCredential.IsNull() && !data.UseDynamicTeamCredential.IsUnknown() {
		useDynamic := data.UseDynamicTeamCredential.ValueBool()
		updateBody.UseDynamicTeamCredential = &useDynamic
	}

	// Set response modifier template if provided
	if !data.ResponseModifierTemplate.IsNull() && !data.ResponseModifierTemplate.IsUnknown() {
		template := data.ResponseModifierTemplate.ValueString()
		updateBody.ResponseModifierTemplate = &template
	}

	// Set allow usage when untrusted data if provided
	if !data.AllowUsageWhenUntrustedData.IsNull() && !data.AllowUsageWhenUntrustedData.IsUnknown() {
		allow := data.AllowUsageWhenUntrustedData.ValueBool()
		updateBody.AllowUsageWhenUntrustedDataIsPresent = &allow
	}

	// Set tool result treatment if provided
	if !data.ToolResultTreatment.IsNull() && !data.ToolResultTreatment.IsUnknown() {
		treatment := data.ToolResultTreatment.ValueString()
		var treatmentEnum client.UpdateAgentToolJSONBodyToolResultTreatment
		switch treatment {
		case "trusted":
			treatmentEnum = client.UpdateAgentToolJSONBodyToolResultTreatmentTrusted
		case "untrusted":
			treatmentEnum = client.UpdateAgentToolJSONBodyToolResultTreatmentUntrusted
		case "sanitize_with_dual_llm":
			treatmentEnum = client.UpdateAgentToolJSONBodyToolResultTreatmentSanitizeWithDualLlm
		default:
			return fmt.Errorf("invalid tool_result_treatment value: %s", treatment)
		}
		updateBody.ToolResultTreatment = &treatmentEnum
	}

	// Call update API
	updateResp, err := r.client.UpdateAgentToolWithResponse(ctx, openapi_types.UUID(agentToolUUID), updateBody)
	if err != nil {
		return fmt.Errorf("unable to update agent tool: %w", err)
	}

	if updateResp.JSON200 == nil {
		if updateResp.JSON400 != nil {
			return fmt.Errorf("failed to update agent tool: %s", updateResp.JSON400.Error.Message)
		}
		return fmt.Errorf("expected 200 OK, got status %d", updateResp.StatusCode())
	}

	return nil
}
