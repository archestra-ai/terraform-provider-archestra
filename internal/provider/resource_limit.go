package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &LimitResource{}
var _ resource.ResourceWithImportState = &LimitResource{}
var _ resource.ResourceWithValidateConfig = &LimitResource{}

func NewLimitResource() resource.Resource {
	return &LimitResource{}
}

// LimitResource defines the resource implementation.
type LimitResource struct {
	client *client.ClientWithResponses
}

// LimitResourceModel describes the resource data model.
type LimitResourceModel struct {
	ID            types.String `tfsdk:"id"`
	EntityID      types.String `tfsdk:"entity_id"`
	EntityType    types.String `tfsdk:"entity_type"`
	LimitType     types.String `tfsdk:"limit_type"`
	LimitValue    types.Int64  `tfsdk:"limit_value"`
	Model         types.List   `tfsdk:"model"`
	ToolName      types.String `tfsdk:"tool_name"`
	MCPServerName types.String `tfsdk:"mcp_server_name"`
}

func (r *LimitResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_limit"
}

func (r *LimitResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages usage limits in Archestra.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Limit identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"entity_id": schema.StringAttribute{
				MarkdownDescription: "The entity ID this limit applies to",
				Required:            true,
			},
			"entity_type": schema.StringAttribute{
				MarkdownDescription: "Entity type: organization, team, or agent",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("organization", "team", "agent"),
				},
			},
			"limit_type": schema.StringAttribute{
				MarkdownDescription: "Limit type: 'token_cost' (requires model), 'tool_calls' (requires mcp_server_name and tool_name), or 'mcp_server_calls' (requires mcp_server_name)",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("token_cost", "tool_calls", "mcp_server_calls"),
				},
			},
			"limit_value": schema.Int64Attribute{
				MarkdownDescription: "Limit threshold value",
				Required:            true,
			},
			"model": schema.ListAttribute{
				MarkdownDescription: "Required when limit_type is 'token_cost'. List of model names this limit applies to.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"tool_name": schema.StringAttribute{
				MarkdownDescription: "Required when limit_type is 'tool_calls'. Name of the tool this limit applies to.",
				Optional:            true,
			},
			"mcp_server_name": schema.StringAttribute{
				MarkdownDescription: "Required when limit_type is 'mcp_server_calls' or 'tool_calls'. Name of the MCP server.",
				Optional:            true,
			},
		},
	}
}

func (r *LimitResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data LimitResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Skip validation if limit_type is unknown (e.g., during plan with variables)
	if data.LimitType.IsUnknown() {
		return
	}

	limitType := data.LimitType.ValueString()

	switch limitType {
	case "token_cost":
		// model must be set and non-empty
		if data.Model.IsNull() || data.Model.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("model"),
				"Missing Required Attribute",
				"model is required when limit_type is 'token_cost'",
			)
		} else {
			var models []string
			data.Model.ElementsAs(ctx, &models, false)
			if len(models) == 0 {
				resp.Diagnostics.AddAttributeError(
					path.Root("model"),
					"Invalid Attribute Value",
					"model must contain at least one value when limit_type is 'token_cost'",
				)
			}
		}
		// mcp_server_name must NOT be set
		if !data.MCPServerName.IsNull() && !data.MCPServerName.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("mcp_server_name"),
				"Invalid Attribute Combination",
				"mcp_server_name must not be set when limit_type is 'token_cost'",
			)
		}
		// tool_name must NOT be set
		if !data.ToolName.IsNull() && !data.ToolName.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("tool_name"),
				"Invalid Attribute Combination",
				"tool_name must not be set when limit_type is 'token_cost'",
			)
		}

	case "mcp_server_calls":
		// mcp_server_name is required
		if data.MCPServerName.IsNull() || data.MCPServerName.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("mcp_server_name"),
				"Missing Required Attribute",
				"mcp_server_name is required when limit_type is 'mcp_server_calls'",
			)
		}
		// model must NOT be set
		if !data.Model.IsNull() && !data.Model.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("model"),
				"Invalid Attribute Combination",
				"model must not be set when limit_type is 'mcp_server_calls'",
			)
		}

	case "tool_calls":
		// mcp_server_name is required
		if data.MCPServerName.IsNull() || data.MCPServerName.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("mcp_server_name"),
				"Missing Required Attribute",
				"mcp_server_name is required when limit_type is 'tool_calls'",
			)
		}
		// tool_name is required
		if data.ToolName.IsNull() || data.ToolName.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("tool_name"),
				"Missing Required Attribute",
				"tool_name is required when limit_type is 'tool_calls'",
			)
		}
		// model must NOT be set
		if !data.Model.IsNull() && !data.Model.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("model"),
				"Invalid Attribute Combination",
				"model must not be set when limit_type is 'tool_calls'",
			)
		}
	}
}

func (r *LimitResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LimitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LimitResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := client.CreateLimitJSONRequestBody{
		EntityId:    data.EntityID.ValueString(),
		EntityType:  client.CreateLimitJSONBodyEntityType(data.EntityType.ValueString()),
		LimitType:   client.CreateLimitJSONBodyLimitType(data.LimitType.ValueString()),
		LimitValue:  int(data.LimitValue.ValueInt64()),
		LastCleanup: nil,
	}

	if !data.Model.IsNull() {
		var models []string
		data.Model.ElementsAs(ctx, &models, false)
		requestBody.Model = &models
	}
	if !data.ToolName.IsNull() {
		toolName := data.ToolName.ValueString()
		requestBody.ToolName = &toolName
	}
	if !data.MCPServerName.IsNull() {
		mcpServerName := data.MCPServerName.ValueString()
		requestBody.McpServerName = &mcpServerName
	}

	apiResp, err := r.client.CreateLimitWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create limit, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.EntityID = types.StringValue(apiResp.JSON200.EntityId)
	data.EntityType = types.StringValue(string(apiResp.JSON200.EntityType))
	data.LimitType = types.StringValue(string(apiResp.JSON200.LimitType))
	data.LimitValue = types.Int64Value(int64(apiResp.JSON200.LimitValue))

	if apiResp.JSON200.Model != nil && len(*apiResp.JSON200.Model) > 0 {
		modelList, diags := types.ListValueFrom(ctx, types.StringType, *apiResp.JSON200.Model)
		resp.Diagnostics.Append(diags...)
		data.Model = modelList
	} else {
		data.Model = types.ListNull(types.StringType)
	}
	if apiResp.JSON200.ToolName != nil {
		data.ToolName = types.StringValue(*apiResp.JSON200.ToolName)
	}
	if apiResp.JSON200.McpServerName != nil {
		data.MCPServerName = types.StringValue(*apiResp.JSON200.McpServerName)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LimitResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse limit ID: %s", err))
		return
	}

	apiResp, err := r.client.GetLimitWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read limit, got error: %s", err))
		return
	}

	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.EntityID = types.StringValue(apiResp.JSON200.EntityId)
	data.EntityType = types.StringValue(string(apiResp.JSON200.EntityType))
	data.LimitType = types.StringValue(string(apiResp.JSON200.LimitType))
	data.LimitValue = types.Int64Value(int64(apiResp.JSON200.LimitValue))

	if apiResp.JSON200.Model != nil && len(*apiResp.JSON200.Model) > 0 {
		modelList, diags := types.ListValueFrom(ctx, types.StringType, *apiResp.JSON200.Model)
		resp.Diagnostics.Append(diags...)
		data.Model = modelList
	} else {
		data.Model = types.ListNull(types.StringType)
	}
	if apiResp.JSON200.ToolName != nil {
		data.ToolName = types.StringValue(*apiResp.JSON200.ToolName)
	} else {
		data.ToolName = types.StringNull()
	}
	if apiResp.JSON200.McpServerName != nil {
		data.MCPServerName = types.StringValue(*apiResp.JSON200.McpServerName)
	} else {
		data.MCPServerName = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data LimitResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse limit ID: %s", err))
		return
	}

	entityID := data.EntityID.ValueString()
	entityType := client.UpdateLimitJSONBodyEntityType(data.EntityType.ValueString())
	limitType := client.UpdateLimitJSONBodyLimitType(data.LimitType.ValueString())
	limitValue := int(data.LimitValue.ValueInt64())

	requestBody := client.UpdateLimitJSONRequestBody{
		EntityId:    &entityID,
		EntityType:  &entityType,
		LimitType:   &limitType,
		LimitValue:  &limitValue,
		LastCleanup: nil,
	}

	if !data.Model.IsNull() {
		var models []string
		data.Model.ElementsAs(ctx, &models, false)
		requestBody.Model = &models
	}
	if !data.ToolName.IsNull() {
		toolName := data.ToolName.ValueString()
		requestBody.ToolName = &toolName
	}
	if !data.MCPServerName.IsNull() {
		mcpServerName := data.MCPServerName.ValueString()
		requestBody.McpServerName = &mcpServerName
	}

	apiResp, err := r.client.UpdateLimitWithResponse(ctx, id, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update limit, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.EntityID = types.StringValue(apiResp.JSON200.EntityId)
	data.EntityType = types.StringValue(string(apiResp.JSON200.EntityType))
	data.LimitType = types.StringValue(string(apiResp.JSON200.LimitType))
	data.LimitValue = types.Int64Value(int64(apiResp.JSON200.LimitValue))

	if apiResp.JSON200.Model != nil && len(*apiResp.JSON200.Model) > 0 {
		modelList, diags := types.ListValueFrom(ctx, types.StringType, *apiResp.JSON200.Model)
		resp.Diagnostics.Append(diags...)
		data.Model = modelList
	} else {
		data.Model = types.ListNull(types.StringType)
	}
	if apiResp.JSON200.ToolName != nil {
		data.ToolName = types.StringValue(*apiResp.JSON200.ToolName)
	} else {
		data.ToolName = types.StringNull()
	}
	if apiResp.JSON200.McpServerName != nil {
		data.MCPServerName = types.StringValue(*apiResp.JSON200.McpServerName)
	} else {
		data.MCPServerName = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LimitResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse limit ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteLimitWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete limit, got error: %s", err))
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

func (r *LimitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
