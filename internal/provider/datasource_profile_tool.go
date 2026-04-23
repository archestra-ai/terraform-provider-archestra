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
	ID        types.String `tfsdk:"id"`
	ProfileID types.String `tfsdk:"profile_id"`
	ToolID    types.String `tfsdk:"tool_id"`
	ToolName  types.String `tfsdk:"tool_name"`
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

	retryConfig := DefaultRetryConfig(fmt.Sprintf("Tool '%s' for profile %s", targetToolName, targetProfileID))

	type profileToolResult struct {
		ID     string
		ToolID string
	}

	profileUUID, err := uuid.Parse(targetProfileID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Could not parse profile ID as UUID: %s", err))
		return
	}

	limit := 100

	result, found, err := RetryUntilFound(ctx, retryConfig, func() (profileToolResult, bool, error) {
		offset := 0
		totalTools := 0
		for {
			toolsResp, err := d.client.GetAllAgentToolsWithResponse(ctx, &client.GetAllAgentToolsParams{
				AgentId: &profileUUID,
				Limit:   &limit,
				Offset:  &offset,
			})
			if err != nil {
				return profileToolResult{}, false, fmt.Errorf("unable to read profile tools: %w", err)
			}

			if toolsResp.JSON200 == nil {
				return profileToolResult{}, false, fmt.Errorf("expected 200 OK, got status %d", toolsResp.StatusCode())
			}

			totalTools = toolsResp.JSON200.Pagination.Total

			for i := range toolsResp.JSON200.Data {
				profileTool := &toolsResp.JSON200.Data[i]
				if profileTool.Tool.Name == targetToolName {
					return profileToolResult{
						ID:     profileTool.Id.String(),
						ToolID: profileTool.Tool.Id,
					}, true, nil
				}
			}

			if !toolsResp.JSON200.Pagination.HasNext {
				break
			}
			offset += limit
		}

		// If the profile has no tools at all, don't retry — nothing will appear asynchronously
		if totalTools == 0 {
			return profileToolResult{}, false, fmt.Errorf("profile %s has no tools assigned", targetProfileID)
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

	data.ID = types.StringValue(result.ID)
	data.ToolID = types.StringValue(result.ToolID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
