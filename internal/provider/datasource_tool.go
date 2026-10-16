package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ToolDataSource{}

func NewToolDataSource() datasource.DataSource {
	return &ToolDataSource{}
}

type ToolDataSource struct {
	client *client.ClientWithResponses
}

type ToolDataSourceModel struct {
	MCPServerID types.String `tfsdk:"mcp_server_id"`
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (d *ToolDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool"
}

func (d *ToolDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a tool by name. This includes built-in tools (e.g., `archestra__whoami`) and MCP server tools.",

		Attributes: map[string]schema.Attribute{
			"mcp_server_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Optional MCP server UUID to scope the exact-name lookup."},
			"id": schema.StringAttribute{
				MarkdownDescription: "Tool identifier",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the tool to look up",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Tool description",
				Computed:            true,
			},
		},
	}
}

func (d *ToolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ToolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ToolDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.MCPServerID.IsNull() {
		d.readServerTool(ctx, req, resp)
		return
	}
	targetName := data.Name.ValueString()

	// Retry to bridge the race between this data source running and a
	// sibling `archestra_mcp_server` registering its tools
	// in the same plan. Tool names are referenced by string in HCL —
	// Terraform's dep graph can't infer the edge automatically, so the
	// data source can otherwise return "not found" before tools are
	// registered. The optional server selector gives Terraform an explicit dependency.
	type toolResult struct {
		ID          string
		Description *string
	}
	retryConfig := DefaultRetryConfig(fmt.Sprintf("Tool '%s'", targetName))
	result, found, err := RetryUntilFound(ctx, retryConfig, func() (toolResult, bool, error) {
		tool, found, err := findToolByName(ctx, d.client, targetName)
		if err != nil {
			return toolResult{}, false, fmt.Errorf("unable to read tools: %w", err)
		}
		if found {
			return toolResult{ID: tool.ID, Description: tool.Description}, true, nil
		}
		return toolResult{}, false, nil
	})

	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("Tool '%s' not found after retry. If the tool comes from an `archestra_mcp_server` resource in the same module, add `depends_on = [archestra_mcp_server.<n>]` on this data source — string-name references don't create an implicit dependency.", targetName),
		)
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

type toolLookup struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

func findToolByName(ctx context.Context, apiClient *client.ClientWithResponses, targetName string) (toolLookup, bool, error) {
	const pageLimit = 100
	matches := map[string]toolLookup{}

	limit := pageLimit
	search := targetName
	for offset := 0; ; offset += pageLimit {
		apiResp, err := apiClient.GetToolsWithAssignmentsWithResponse(ctx, &client.GetToolsWithAssignmentsParams{
			Search: &search,
			Limit:  &limit,
			Offset: &offset,
		})
		if err != nil {
			return toolLookup{}, false, err
		}
		if apiResp.JSON200 == nil {
			return toolLookup{}, false, fmt.Errorf("expected 200 OK, got status %d", apiResp.StatusCode())
		}

		for _, tool := range apiResp.JSON200.Data {
			if tool.Name == targetName {
				matches[tool.Id] = toolLookup{ID: tool.Id, Name: tool.Name, Description: tool.Description}
			}
		}
		if !apiResp.JSON200.Pagination.HasNext {
			break
		}
		if len(apiResp.JSON200.Data) == 0 {
			return toolLookup{}, false, fmt.Errorf("tool listing returned an empty page with more results")
		}
	}
	if len(matches) > 1 {
		return toolLookup{}, false, fmt.Errorf("multiple tools have the exact name %q; specify mcp_server_id", targetName)
	}
	for _, tool := range matches {
		return tool, true, nil
	}
	return toolLookup{}, false, nil
}
