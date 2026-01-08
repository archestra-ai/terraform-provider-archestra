package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &RolesDataSource{}

func NewRolesDataSource() datasource.DataSource {
	return &RolesDataSource{}
}

type RolesDataSource struct {
	client *client.ClientWithResponses
}

type RolesDataSourceModel struct {
	Roles []RoleModel `tfsdk:"roles"`
}

type RoleModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Role           types.String `tfsdk:"role"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Predefined     types.Bool   `tfsdk:"predefined"`
	Permission     types.Map    `tfsdk:"permission"`
}

func (d *RolesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_roles"
}

func (d *RolesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches all roles in the organization (both predefined and custom).",

		Attributes: map[string]schema.Attribute{
			"roles": schema.ListNestedAttribute{
				MarkdownDescription: "List of all roles",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Role identifier",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The display name of the role",
							Computed:            true,
						},
						"role": schema.StringAttribute{
							MarkdownDescription: "The immutable role identifier",
							Computed:            true,
						},
						"organization_id": schema.StringAttribute{
							MarkdownDescription: "The organization ID this role belongs to",
							Computed:            true,
						},
						"predefined": schema.BoolAttribute{
							MarkdownDescription: "Whether the role is a predefined (immutable) role",
							Computed:            true,
						},
						"permission": schema.MapAttribute{
							MarkdownDescription: "Map of resource names to allowed actions",
							Computed:            true,
							ElementType: types.ListType{
								ElemType: types.StringType,
							},
						},
					},
				},
			},
		},
	}
}

func (d *RolesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RolesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API
	apiResp, err := d.client.GetRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list roles, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	// Map response to Terraform state
	data.Roles = make([]RoleModel, len(*apiResp.JSON200))
	for i, role := range *apiResp.JSON200 {
		var orgId types.String
		if role.OrganizationId != nil {
			orgId = types.StringValue(*role.OrganizationId)
		} else {
			orgId = types.StringNull()
		}

		data.Roles[i] = RoleModel{
			ID:             types.StringValue(role.Id),
			Name:           types.StringValue(role.Name),
			Role:           types.StringValue(role.Role),
			OrganizationID: orgId,
			Predefined:     types.BoolValue(role.Predefined),
			Permission:     convertGetRolesPermissionToTerraform(role.Permission),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// convertGetRolesPermissionToTerraform converts GetRoles API permission response to Terraform map
func convertGetRolesPermissionToTerraform(permission map[string][]client.GetRoles200Permission) types.Map {
	elements := make(map[string]attr.Value)
	for resource, actions := range permission {
		actionValues := make([]attr.Value, len(actions))
		for i, action := range actions {
			actionValues[i] = types.StringValue(string(action))
		}
		elements[resource] = types.ListValueMust(types.StringType, actionValues)
	}

	return types.MapValueMust(types.ListType{ElemType: types.StringType}, elements)
}
