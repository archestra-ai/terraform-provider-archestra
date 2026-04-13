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

var _ datasource.DataSource = &MCPServerToolDataSource{}

func NewMCPServerToolDataSource() datasource.DataSource {
	return &MCPServerToolDataSource{}
}

type MCPServerToolDataSource struct {
	client *client.ClientWithResponses
}

type MCPServerToolDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	MCPServerID types.String `tfsdk:"mcp_server_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (d *MCPServerToolDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server_tool"
}

func (d *MCPServerToolDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a tool from an MCP server by MCP server ID and tool name. " +
			"This data source is useful for looking up tools provided by MCP servers.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Tool identifier",
				Computed:            true,
			},
			"mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "The MCP server ID",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the tool",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Tool description",
				Computed:            true,
			},
		},
	}
}

func (d *MCPServerToolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *MCPServerToolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MCPServerToolDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mcpServerID, err := uuid.Parse(data.MCPServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid MCP Server ID", fmt.Sprintf("Unable to parse MCP server ID as UUID: %s", err))
		return
	}

	targetToolName := data.Name.ValueString()
	retryConfig := DefaultRetryConfig(fmt.Sprintf("Tool '%s' for MCP server %s", targetToolName, data.MCPServerID.ValueString()))

	type mcpToolResult struct {
		ID          string
		Description *string
	}

	result, found, err := RetryUntilFound(ctx, retryConfig, func() (mcpToolResult, bool, error) {
		toolsResp, err := d.client.GetMcpServerToolsWithResponse(ctx, mcpServerID)
		if err != nil {
			return mcpToolResult{}, false, fmt.Errorf("unable to read MCP server tools: %w", err)
		}

		if toolsResp.JSON200 == nil {
			return mcpToolResult{}, false, fmt.Errorf("expected 200 OK, got status %d", toolsResp.StatusCode())
		}

		for i := range *toolsResp.JSON200 {
			tool := &(*toolsResp.JSON200)[i]
			if tool.Name == targetToolName {
				return mcpToolResult{
					ID:          tool.Id,
					Description: tool.Description,
				}, true, nil
			}
		}

		return mcpToolResult{}, false, nil
	})

	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	if !found {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Tool '%s' not found for MCP server %s", targetToolName, data.MCPServerID.ValueString()))
		return
	}

	data.ID = types.StringValue(result.ID)
	if result.Description != nil {
		data.Description = types.StringValue(*result.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
