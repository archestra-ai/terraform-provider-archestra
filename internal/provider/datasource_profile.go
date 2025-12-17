package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ProfileDataSource{}

func NewProfileDataSource() datasource.DataSource {
	return &ProfileDataSource{}
}

type ProfileDataSource struct {
	client *client.ClientWithResponses
}

type ProfileDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (d *ProfileDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile"
}

func (d *ProfileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra profile (agent) by name. Profiles are also known as agents in the Archestra API.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Profile identifier (use this for profile_id in archestra_profile_tool)",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the profile",
				Required:            true,
			},
		},
	}
}

func (d *ProfileDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ProfileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProfileDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	targetName := data.Name.ValueString()

	// Get all agents and find by name
	agentsResp, err := d.client.GetAgentsWithResponse(ctx, &client.GetAgentsParams{})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agents: %s", err))
		return
	}

	if agentsResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", agentsResp.StatusCode()))
		return
	}

	// Find agent by name
	var foundID string
	for _, agent := range agentsResp.JSON200.Data {
		if agent.Name == targetName {
			foundID = agent.Id.String()
			break
		}
	}

	if foundID == "" {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Profile with name '%s' not found", targetName))
		return
	}

	// Map to state
	data.ID = types.StringValue(foundID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
