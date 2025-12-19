package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ProfileToolDataSource{}

func NewProfileToolDataSource() datasource.DataSource {
	return &ProfileToolDataSource{}
}

type ProfileToolDataSource struct {
	client *client.ClientWithResponses
}

type ProfileToolDataSourceModel struct {
	ID                                   types.String `tfsdk:"id"`
	ProfileID                            types.String `tfsdk:"profile_id"`
	ToolID                               types.String `tfsdk:"tool_id"`
	ToolName                             types.String `tfsdk:"tool_name"`
	AllowUsageWhenUntrustedDataIsPresent types.Bool   `tfsdk:"allow_usage_when_untrusted_data_is_present"`
	ToolResultTreatment                  types.String `tfsdk:"tool_result_treatment"`
	ResponseModifierTemplate             types.String `tfsdk:"response_modifier_template"`
}

func (d *ProfileToolDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile_tool"
}

func (d *ProfileToolDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a profile tool by profile ID and tool name. This data source is useful for " +
			"looking up the profile_tool_id needed to create trusted data policies and tool invocation policies.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Profile tool identifier (use this for policy profile_tool_id)",
				Computed:            true,
			},
			"profile_id": schema.StringAttribute{
				MarkdownDescription: "The profile ID",
				Required:            true,
			},
			"tool_name": schema.StringAttribute{
				MarkdownDescription: "The name of the tool",
				Required:            true,
			},
			"tool_id": schema.StringAttribute{
				MarkdownDescription: "The tool ID",
				Computed:            true,
			},
			"allow_usage_when_untrusted_data_is_present": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow tool usage when untrusted data is present",
				Computed:            true,
			},
			"tool_result_treatment": schema.StringAttribute{
				MarkdownDescription: "How to treat tool results (trusted/untrusted)",
				Computed:            true,
			},
			"response_modifier_template": schema.StringAttribute{
				MarkdownDescription: "Optional response modifier template",
				Computed:            true,
			},
		},
	}
}

func (d *ProfileToolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *ProfileToolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProfileToolDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	targetProfileID := data.ProfileID.ValueString()
	targetToolName := data.ToolName.ValueString()

	// Use retry logic for built-in tools that may not be immediately available after profile creation.
	// Built-in tools like "archestra__whoami" are assigned asynchronously.
	retryConfig := DefaultRetryConfig(fmt.Sprintf("Tool '%s' for profile %s", targetToolName, targetProfileID))

	// profileToolResult holds the extracted data we need from the API response
	type profileToolResult struct {
		ID                                   string
		ToolID                               string
		AllowUsageWhenUntrustedDataIsPresent bool
		ToolResultTreatment                  string
		ResponseModifierTemplate             *string
	}

	// Parse profile ID as UUID for the API filter
	// Note: API still uses "Agent" terminology
	profileUUID, err := uuid.Parse(targetProfileID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Could not parse profile ID as UUID: %s", err))
		return
	}

	// Use max allowed limit to get all tools for this profile in one request
	// (built-in tools are typically <30, so 100 is more than enough)
	limit := 100

	result, found, err := RetryUntilFound(ctx, retryConfig, func() (profileToolResult, bool, error) {
		// Get agent tools filtered by agent ID (more efficient than fetching all)
		toolsResp, err := d.client.GetAllAgentToolsWithResponse(ctx, &client.GetAllAgentToolsParams{
			AgentId: &profileUUID,
			Limit:   &limit,
		})
		if err != nil {
			return profileToolResult{}, false, fmt.Errorf("unable to read profile tools: %w", err)
		}

		if toolsResp.JSON200 == nil {
			return profileToolResult{}, false, fmt.Errorf("expected 200 OK, got status %d", toolsResp.StatusCode())
		}

		// Find the specific tool by name
		for i := range toolsResp.JSON200.Data {
			agentTool := &toolsResp.JSON200.Data[i]
			if agentTool.Tool.Name == targetToolName {
				return profileToolResult{
					ID:                                   agentTool.Id.String(),
					ToolID:                               agentTool.Tool.Id,
					AllowUsageWhenUntrustedDataIsPresent: agentTool.AllowUsageWhenUntrustedDataIsPresent,
					ToolResultTreatment:                  string(agentTool.ToolResultTreatment),
					ResponseModifierTemplate:             agentTool.ResponseModifierTemplate,
				}, true, nil
			}
		}

		return profileToolResult{}, false, nil
	})

	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	if !found {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Tool '%s' not found for profile %s", targetToolName, targetProfileID))
		return
	}

	// Map to state
	data.ID = types.StringValue(result.ID)
	data.ToolID = types.StringValue(result.ToolID)
	data.AllowUsageWhenUntrustedDataIsPresent = types.BoolValue(result.AllowUsageWhenUntrustedDataIsPresent)
	data.ToolResultTreatment = types.StringValue(result.ToolResultTreatment)

	if result.ResponseModifierTemplate != nil {
		data.ResponseModifierTemplate = types.StringValue(*result.ResponseModifierTemplate)
	} else {
		data.ResponseModifierTemplate = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
