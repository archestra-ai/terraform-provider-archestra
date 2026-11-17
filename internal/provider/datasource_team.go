package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &TeamDataSource{}

func NewTeamDataSource() datasource.DataSource {
	return &TeamDataSource{}
}

type TeamDataSource struct {
	client *client.ClientWithResponses
}

type TeamDataSourceModel struct {
	ExternalGroupIDs types.Set         `tfsdk:"external_group_ids"`
	ID               types.String      `tfsdk:"id"`
	Name             types.String      `tfsdk:"name"`
	Description      types.String      `tfsdk:"description"`
	OrganizationID   types.String      `tfsdk:"organization_id"`
	CreatedBy        types.String      `tfsdk:"created_by"`
	Members          []TeamMemberModel `tfsdk:"members"`
}

func (d *TeamDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (d *TeamDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra team by `id` or by its exact `name`. Set one of the two.",

		Attributes: map[string]schema.Attribute{
			"external_group_ids": schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "SSO groups mapped to this team, when visible and Team Sync is available."},
			"id": schema.StringAttribute{
				MarkdownDescription: "Team identifier. Set this or `name`.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the team. Set it to find the team by its exact name instead of by `id`.",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the team",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID this team belongs to",
				Computed:            true,
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "User ID of the team creator",
				Computed:            true,
			},
			"members": schema.ListNestedAttribute{
				MarkdownDescription: "List of team members",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"user_id": schema.StringAttribute{
							MarkdownDescription: "User ID of the team member",
							Computed:            true,
						},
						"role": schema.StringAttribute{
							MarkdownDescription: "Role of the team member",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *TeamDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TeamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TeamDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.ValueString() == "" {
		id, ok := d.teamIDByName(ctx, data.Name.ValueString(), resp)
		if !ok {
			return
		}
		data.ID = types.StringValue(id)
	}

	// Get team data
	teamResp, err := d.client.GetTeamWithResponse(ctx, data.ID.ValueString(), nil)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read team, got error: %s", err))
		return
	}

	if teamResp.JSON404 != nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Team with ID %s not found", data.ID.ValueString()))
		return
	}

	if teamResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", teamResp.StatusCode()))
		return
	}

	team := teamResp.JSON200
	data.Name = types.StringValue(team.Name)
	data.OrganizationID = types.StringValue(team.OrganizationId)
	data.CreatedBy = types.StringPointerValue(team.CreatedBy)
	if team.Description != nil {
		data.Description = types.StringValue(*team.Description)
	} else {
		data.Description = types.StringNull()
	}

	// Fetch team members
	membersResp, err := d.client.GetTeamMembersWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read team members, got error: %s", err))
		return
	}

	if membersResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", membersResp.StatusCode()))
		return
	}

	data.Members = make([]TeamMemberModel, len(*membersResp.JSON200))
	for i, member := range *membersResp.JSON200 {
		data.Members[i] = TeamMemberModel{
			UserID: types.StringValue(member.UserId),
			Role:   types.StringValue(member.Role),
		}
	}

	view := TeamResourceModel{ID: data.ID, ExternalGroupIDs: types.SetNull(types.StringType)}
	(&TeamResource{client: d.client}).readBindings(ctx, &view, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ExternalGroupIDs = view.ExternalGroupIDs
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// teamIDByName finds the one team with exactly this name. The API's name
// filter matches part of a name, so the exact match is made here.
func (d *TeamDataSource) teamIDByName(ctx context.Context, name string, resp *datasource.ReadResponse) (string, bool) {
	if name == "" {
		resp.Diagnostics.AddError("Missing Lookup", "Set id or name.")
		return "", false
	}
	limit, includeRelations := 100, false
	found := ""
	for offset := 0; ; offset += limit {
		teams, err := d.client.GetTeamsWithResponse(ctx, &client.GetTeamsParams{Name: &name, Limit: &limit, Offset: &offset, IncludeRelations: &includeRelations})
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list teams: %s", err))
			return "", false
		}
		if teams.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", teams.StatusCode()))
			return "", false
		}
		for _, team := range teams.JSON200.Data {
			if team.Name != name {
				continue
			}
			if found != "" {
				resp.Diagnostics.AddError("Ambiguous Team", fmt.Sprintf("Several teams are named %q. Look the team up by id.", name))
				return "", false
			}
			found = team.Id
		}
		if len(teams.JSON200.Data) < limit {
			break
		}
	}
	if found == "" {
		resp.Diagnostics.AddError("Team Not Found", fmt.Sprintf("No team visible to this API key is named %q.", name))
		return "", false
	}
	return found, true
}
