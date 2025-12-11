package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &AgentToolDataSource{}

func NewAgentToolDataSource() datasource.DataSource {
	return &AgentToolDataSource{}
}

type AgentToolDataSource struct {
	client *client.ClientWithResponses
}

type AgentToolDataSourceModel struct {
	ID                                   types.String `tfsdk:"id"`
	AgentID                              types.String `tfsdk:"agent_id"`
	ToolID                               types.String `tfsdk:"tool_id"`
	ToolName                             types.String `tfsdk:"tool_name"`
	AllowUsageWhenUntrustedDataIsPresent types.Bool   `tfsdk:"allow_usage_when_untrusted_data_is_present"`
	ToolResultTreatment                  types.String `tfsdk:"tool_result_treatment"`
	ResponseModifierTemplate             types.String `tfsdk:"response_modifier_template"`
}

func (d *AgentToolDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_tool"
}

func (d *AgentToolDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an agent tool by agent ID and tool name. This data source is useful for " +
			"looking up the agent_tool_id needed to create trusted data policies and tool invocation policies.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Agent tool identifier (use this for policy agent_tool_id)",
				Computed:            true,
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "The agent ID",
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

func (d *AgentToolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AgentToolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentToolDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	targetAgentID := data.AgentID.ValueString()
	targetToolName := data.ToolName.ValueString()

	// Use retry logic for built-in tools that may not be immediately available after agent creation.
	// Built-in tools like "archestra__whoami" are assigned asynchronously.
	retryConfig := DefaultRetryConfig(fmt.Sprintf("Tool '%s' for agent %s", targetToolName, targetAgentID))

	// agentToolResult holds the extracted data we need from the API response
	type agentToolResult struct {
		ID                                   string
		ToolID                               string
		AllowUsageWhenUntrustedDataIsPresent bool
		ToolResultTreatment                  string
		ResponseModifierTemplate             *string
	}

	result, found, err := RetryUntilFound(ctx, retryConfig, func() (agentToolResult, bool, error) {
		// Get all agent tools (which includes configuration)
		toolsResp, err := d.client.GetAllAgentToolsWithResponse(ctx, nil)
		if err != nil {
			return agentToolResult{}, false, fmt.Errorf("unable to read agent tools: %w", err)
		}

		if toolsResp.JSON200 == nil {
			return agentToolResult{}, false, fmt.Errorf("expected 200 OK, got status %d", toolsResp.StatusCode())
		}

		// Filter by agent ID and tool name
		for i := range toolsResp.JSON200.Data {
			agentTool := &toolsResp.JSON200.Data[i]
			if agentTool.Agent.Id == targetAgentID && agentTool.Tool.Name == targetToolName {
				return agentToolResult{
					ID:                                   agentTool.Id.String(),
					ToolID:                               agentTool.Tool.Id,
					AllowUsageWhenUntrustedDataIsPresent: agentTool.AllowUsageWhenUntrustedDataIsPresent,
					ToolResultTreatment:                  string(agentTool.ToolResultTreatment),
					ResponseModifierTemplate:             agentTool.ResponseModifierTemplate,
				}, true, nil
			}
		}

		return agentToolResult{}, false, nil
	})

	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	if !found {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Tool '%s' not found for agent %s", targetToolName, targetAgentID))
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
