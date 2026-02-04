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
	Name        types.String `tfsdk:"name"`
	ID          types.String `tfsdk:"id"`
	Permissions types.List   `tfsdk:"permissions"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing Archestra role by name.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role to look up",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier",
				Computed:            true,
			},
			"permissions": schema.ListAttribute{
				MarkdownDescription: "List of permissions in format 'resource:action'",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *RoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
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
	var found bool
	var foundId string
	var foundPermissions map[string][]client.GetRoles200Permission

	for _, role := range *apiResp.JSON200 {
		if role.Name == data.Name.ValueString() {
			found = true
			foundId = role.Id
			foundPermissions = role.Permission
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Role Not Found",
			fmt.Sprintf("No role found with name: %s", data.Name.ValueString()),
		)
		return
	}

	// Set data
	data.ID = types.StringValue(foundId)

	// Convert permissions map to list
	permList := []string{}
	for resource, actions := range foundPermissions {
		for _, action := range actions {
			permList = append(permList, fmt.Sprintf("%s:%s", resource, string(action)))
		}
	}

	permListValue, diags := types.ListValueFrom(ctx, types.StringType, permList)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = permListValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
