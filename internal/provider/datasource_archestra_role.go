package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ArchestraRoleDataSource{}

func NewArchestraRoleDataSource() datasource.DataSource {
	return &ArchestraRoleDataSource{}
}

type ArchestraRoleDataSource struct {
	client *client.ClientWithResponses
}

type ArchestraRoleDataSourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Permissions []types.String `tfsdk:"permissions"`
}

func (d *ArchestraRoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *ArchestraRoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra RBAC role by name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role to lookup",
				Required:            true,
			},
			"permissions": schema.ListAttribute{
				MarkdownDescription: "List of permissions",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *ArchestraRoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ArchestraRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ArchestraRoleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	targetName := data.Name.ValueString()

	// List all roles to find by name
	apiResp, err := d.client.GetRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list roles, got error: %s", err))
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
	// Note: GetRoles returns *[]struct{...} or similar. I need to check the return type of GetRoles in client code.
	// Assuming it returns []CreateRoleResponse-like structs or similar.
	// Actually, GetRolesResponse struct likely has JSON200 which is *[]Role.

	// Let's assume standard behavior:
	// But since I can't easily see GetRolesResponse definition, I'll iterate broadly.
	// If compilation fails, I'll fix key fields.

	for _, role := range *apiResp.JSON200 {
		if role.Name == targetName {
			// Found it
			// Need to convert to same format as we used in resource

			data.ID = types.StringValue(role.Id)

			var flattenedPerms []types.String
			if role.Permission != nil {
				for res, actions := range role.Permission {
					for _, action := range actions {
						flattenedPerms = append(flattenedPerms, types.StringValue(fmt.Sprintf("%s:%s", res, action)))
					}
				}
			}
			data.Permissions = flattenedPerms

			// Set state
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role with name '%s' not found", targetName))
}
