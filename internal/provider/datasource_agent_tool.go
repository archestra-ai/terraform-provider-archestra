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

var _ datasource.DataSource = &AgentToolDataSource{}

func NewAgentToolDataSource() datasource.DataSource {
	return &AgentToolDataSource{}
}

type AgentToolDataSource struct {
	client *client.ClientWithResponses
}

type AgentToolDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	AgentID  types.String `tfsdk:"agent_id"`
	ToolID   types.String `tfsdk:"tool_id"`
	ToolName types.String `tfsdk:"tool_name"`
}

func (d *AgentToolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_tool"
}

func (d *AgentToolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches one agent-tool assignment by agent ID and tool name.\n\n" +
			"~> **Listing every tool assigned to an agent?** Use [`data.archestra_agent_tools`](agent_tools) (plural) — returns the full list in one call.\n\n" +
			"~> **Picking the right ID for policies.** `archestra_tool_invocation_policy.tool_id` and `archestra_trusted_data_policy.tool_id` expect the bare tool UUID (`tool_id` field below) — not the assignment composite (`id` field below).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Agent-tool assignment composite UUID. **Not** what `archestra_tool_invocation_policy.tool_id` / `archestra_trusted_data_policy.tool_id` expect — use the `tool_id` field below for those.",
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
				MarkdownDescription: "The bare tool UUID. Pass this as `tool_id` on `archestra_tool_invocation_policy` / `archestra_trusted_data_policy`.",
				Computed:            true,
			},
		},
	}
}

func (d *AgentToolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *AgentToolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentToolDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	targetAgentID := data.AgentID.ValueString()
	targetToolName := data.ToolName.ValueString()

	retryConfig := DefaultRetryConfig(fmt.Sprintf("Tool '%s' for agent %s", targetToolName, targetAgentID))

	type agentToolResult struct {
		ID     string
		ToolID string
	}

	agentUUID, err := uuid.Parse(targetAgentID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", fmt.Sprintf("Could not parse agent_id as UUID: %s", err))
		return
	}

	limit := 100

	result, found, err := RetryUntilFound(ctx, retryConfig, func() (agentToolResult, bool, error) {
		offset := 0
		totalTools := 0
		for {
			toolsResp, err := d.client.GetAllAgentToolsWithResponse(ctx, &client.GetAllAgentToolsParams{
				AgentId: &agentUUID,
				Limit:   &limit,
				Offset:  &offset,
			})
			if err != nil {
				return agentToolResult{}, false, fmt.Errorf("unable to read agent tools: %w", err)
			}

			if toolsResp.JSON200 == nil {
				return agentToolResult{}, false, fmt.Errorf("expected 200 OK, got status %d", toolsResp.StatusCode())
			}

			totalTools = toolsResp.JSON200.Pagination.Total

			for i := range toolsResp.JSON200.Data {
				at := &toolsResp.JSON200.Data[i]
				if at.Tool.Name == targetToolName {
					return agentToolResult{ID: at.Id.String(), ToolID: at.Tool.Id}, true, nil
				}
			}

			if !toolsResp.JSON200.Pagination.HasNext {
				break
			}
			offset += limit
		}

		// If the agent has no tools at all, don't retry — nothing will appear asynchronously
		if totalTools == 0 {
			return agentToolResult{}, false, fmt.Errorf("agent %s has no tools assigned", targetAgentID)
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

	data.ID = types.StringValue(result.ID)
	data.ToolID = types.StringValue(result.ToolID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
