package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &TeamExternalGroupsDataSource{}

func NewTeamExternalGroupsDataSource() datasource.DataSource {
	return &TeamExternalGroupsDataSource{}
}

type TeamExternalGroupsDataSource struct {
	client *client.ClientWithResponses
}


// ---------------------
// Data Source Schema Models
// ---------------------

type TeamExternalGroupItem struct {
	ID              types.String `tfsdk:"id"`
	GroupIdentifier types.String `tfsdk:"group_identifier"`
	TeamID          types.String `tfsdk:"team_id"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

type TeamExternalGroupsDataSourceModel struct {
	TeamID types.String            `tfsdk:"team_id"`
	Groups []TeamExternalGroupItem `tfsdk:"groups"`
}

// ---------------------
// Metadata
// ---------------------

func (d *TeamExternalGroupsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_external_groups"
}

// ---------------------
// Schema
// ---------------------

func (d *TeamExternalGroupsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all external groups synced to an Archestra team.",

		Attributes: map[string]schema.Attribute{
			"team_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the team whose external groups will be listed.",
			},

			"groups": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of external groups associated with the team.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"group_identifier": schema.StringAttribute{
							Computed: true,
						},
						"team_id": schema.StringAttribute{
							Computed: true,
						},
						"created_at": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

// ---------------------
// Configure
// ---------------------

func (d *TeamExternalGroupsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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


// ---------------------
// Read
// ---------------------

func (d *TeamExternalGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TeamExternalGroupsDataSourceModel

	// Load the team_id from the config
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID := data.TeamID.ValueString()

	// Call the API to get all external groups
	apiResp, err := d.client.GetTeamExternalGroupsWithResponse(ctx, teamID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to fetch external groups: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map API response into the Terraform model
	groups := make([]TeamExternalGroupItem, len(*apiResp.JSON200))
	for i, g := range *apiResp.JSON200 {
		groups[i] = TeamExternalGroupItem{
			ID:              types.StringValue(g.Id),
			GroupIdentifier: types.StringValue(g.GroupIdentifier),
			TeamID:          types.StringValue(g.TeamId),
			CreatedAt:       types.StringValue(g.CreatedAt.Format(time.RFC3339)),
		}
	}

	data.Groups = groups

	// Save the data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
