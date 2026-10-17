package provider

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (d *ToolDataSource) readServerTool(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ToolDataSourceModel

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
