package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &RoleDataSource{}

func NewRoleDataSource() datasource.DataSource {
	return &RoleDataSource{}
}

type RoleDataSource struct {
	client *client.ClientWithResponses
}

type RoleDataSourceModel struct {
	ID             types.String          `tfsdk:"id"`
	Name           types.String          `tfsdk:"name"`
	Permissions    map[string]types.List `tfsdk:"permissions"`
	Predefined     types.Bool            `tfsdk:"predefined"`
	OrganizationID types.String          `tfsdk:"organization_id"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing RBAC role by name. Can be used to reference system/predefined roles or custom roles.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role to look up",
				Required:            true,
			},
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Map of resource names to lists of allowed actions",
				Computed:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
			},
			"predefined": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a predefined system role",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Organization ID this role belongs to",
				Computed:            true,
			},
		},
	}
}

func (d *RoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	apiClient, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = apiClient
}

func (d *RoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RoleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get all roles and find by name
	apiResp, err := d.client.GetRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read roles, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Find role by name
	roleName := data.Name.ValueString()
	var found bool
	for _, role := range *apiResp.JSON200 {
		if role.Name == roleName {
			found = true
			data.ID = types.StringValue(role.Id)
			data.Name = types.StringValue(role.Name)
			data.Predefined = types.BoolValue(role.Predefined)
			if role.OrganizationId != nil {
				data.OrganizationID = types.StringValue(*role.OrganizationId)
			} else {
				data.OrganizationID = types.StringNull()
			}

			// Convert permissions
			data.Permissions, resp.Diagnostics = convertGetRolesPermissions(ctx, role.Permission, resp.Diagnostics)
			if resp.Diagnostics.HasError() {
				return
			}
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Role Not Found",
			fmt.Sprintf("Role with name %q was not found", roleName),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
