package provider

import (
	"context"
	"encoding/json"
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

type RoleDataSourcePermissionModel struct {
	Resource    types.String   `tfsdk:"resource"`
	Permissions []types.String `tfsdk:"permissions"`
}

type RoleDataSourceModel struct {
	ID          types.String                    `tfsdk:"id"`
	Name        types.String                    `tfsdk:"name"`
	Permissions []RoleDataSourcePermissionModel `tfsdk:"permissions"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra role by ID or name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier. Either id or name must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role. Either id or name must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"permissions": schema.ListNestedAttribute{
				MarkdownDescription: "List of permissions for this role",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource": schema.StringAttribute{
							MarkdownDescription: "Resource type",
							Computed:            true,
						},
						"permissions": schema.ListAttribute{
							MarkdownDescription: "List of permissions for this resource",
							Computed:            true,
							ElementType:         types.StringType,
						},
					},
				},
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
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
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

	// Validate that either ID or Name is provided
	if data.ID.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either 'id' or 'name' must be specified",
		)
		return
	}

	// If name is provided but not ID, we need to list all roles and find by name
	if !data.Name.IsNull() && data.ID.IsNull() {
		targetName := data.Name.ValueString()

		// Get all roles
		rolesResp, err := d.client.GetRolesWithResponse(ctx)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read roles, got error: %s", err))
			return
		}

		if rolesResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", rolesResp.StatusCode()))
			return
		}

		// Find role by name
		var foundRole *struct {
			CreatedAt      string                                    `json:"createdAt"`
			Id             string                                    `json:"id"`
			Name           string                                    `json:"name"`
			OrganizationId *string                                   `json:"organizationId,omitempty"`
			Permission     map[string][]client.GetRoles200Permission `json:"permission"`
			Predefined     bool                                      `json:"predefined"`
			Role           string                                    `json:"role"`
			UpdatedAt      *string                                   `json:"updatedAt"`
		}
		for i := range *rolesResp.JSON200 {
			if (*rolesResp.JSON200)[i].Name == targetName {
				role := (*rolesResp.JSON200)[i]
				foundRole = &struct {
					CreatedAt      string                                    `json:"createdAt"`
					Id             string                                    `json:"id"`
					Name           string                                    `json:"name"`
					OrganizationId *string                                   `json:"organizationId,omitempty"`
					Permission     map[string][]client.GetRoles200Permission `json:"permission"`
					Predefined     bool                                      `json:"predefined"`
					Role           string                                    `json:"role"`
					UpdatedAt      *string                                   `json:"updatedAt"`
				}{
					CreatedAt:      role.CreatedAt.String(),
					Id:             role.Id,
					Name:           role.Name,
					OrganizationId: role.OrganizationId,
					Permission:     role.Permission,
					Predefined:     role.Predefined,
					Role:           role.Role,
				}
				break
			}
		}

		if foundRole == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role with name '%s' not found", targetName))
			return
		}

		// Map to data model
		data.ID = types.StringValue(foundRole.Id)
		data.Name = types.StringValue(foundRole.Name)

		// Map permissions
		var tfPermissions []RoleDataSourcePermissionModel
		for resource, perms := range foundRole.Permission {
			var tfPerms []types.String
			for _, p := range perms {
				tfPerms = append(tfPerms, types.StringValue(string(p)))
			}
			tfPermissions = append(tfPermissions, RoleDataSourcePermissionModel{
				Resource:    types.StringValue(resource),
				Permissions: tfPerms,
			})
		}
		data.Permissions = tfPermissions

	} else {
		// Get role by ID - GetRole takes a union type parameter
		// The parameter is defined as: roleId struct { Union json.RawMessage }
		// We need to look at how the client methods expect this to be called
		// Since it's a union of GetRoleParamsRoleId0 (enum) or GetRoleParamsRoleId1 (string)
		// We'll pass the ID as a string directly
		roleResp, err := d.client.GetRoleWithResponse(ctx, struct{ Union json.RawMessage }{Union: json.RawMessage(data.ID.ValueString())})
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
			return
		}

		if roleResp.JSON404 != nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role with ID %s not found", data.ID.ValueString()))
			return
		}

		if roleResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", roleResp.StatusCode()))
			return
		}

		// Map to data model
		data.Name = types.StringValue(roleResp.JSON200.Name)

		// Map permissions
		var tfPermissions []RoleDataSourcePermissionModel
		for resource, perms := range roleResp.JSON200.Permission {
			var tfPerms []types.String
			for _, p := range perms {
				tfPerms = append(tfPerms, types.StringValue(string(p)))
			}
			tfPermissions = append(tfPermissions, RoleDataSourcePermissionModel{
				Resource:    types.StringValue(resource),
				Permissions: tfPerms,
			})
		}
		data.Permissions = tfPermissions
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
