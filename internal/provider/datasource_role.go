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
	Role           types.String          `tfsdk:"role"`
	Permissions    map[string]types.List `tfsdk:"permissions"`
	Predefined     types.Bool            `tfsdk:"predefined"`
	OrganizationID types.String          `tfsdk:"organization_id"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra role by name or ID. Use this to look up existing system roles or custom roles.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier (UUID). Either id or name must be provided.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the role. Either id or name must be provided.",
				Optional:            true,
				Computed:            true,
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "Role slug/identifier",
				Computed:            true,
			},
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Map of resource types to permission actions",
				Computed:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
			},
			"predefined": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a system-defined role",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID this role belongs to (null for predefined roles)",
				Computed:            true,
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

	// Validate that at least one identifier is provided
	if data.ID.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either 'id' or 'name' must be provided to look up a role.",
		)
		return
	}

	// Get all roles
	apiResp, err := d.client.GetRolesWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read roles, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	// Find the role by ID or name
	var found bool
	for _, role := range *apiResp.JSON200 {
		// Match by ID if provided
		if !data.ID.IsNull() && role.Id == data.ID.ValueString() {
			found = true
		}
		// Match by name if provided (and ID not provided or also matches)
		if !data.Name.IsNull() && role.Name == data.Name.ValueString() {
			if data.ID.IsNull() || role.Id == data.ID.ValueString() {
				found = true
			}
		}
		// Also match by role slug
		if !data.Name.IsNull() && role.Role == data.Name.ValueString() {
			if data.ID.IsNull() || role.Id == data.ID.ValueString() {
				found = true
			}
		}

		if found {
			data.ID = types.StringValue(role.Id)
			data.Name = types.StringValue(role.Name)
			data.Role = types.StringValue(role.Role)
			data.Predefined = types.BoolValue(role.Predefined)
			if role.OrganizationId != nil {
				data.OrganizationID = types.StringValue(*role.OrganizationId)
			} else {
				data.OrganizationID = types.StringNull()
			}

			// Convert permissions
			tfPermissions := make(map[string]types.List)
			for resourceType, perms := range role.Permission {
				permStrings := make([]string, len(perms))
				for i, p := range perms {
					permStrings[i] = string(p)
				}
				permList, diags := types.ListValueFrom(ctx, types.StringType, permStrings)
				resp.Diagnostics.Append(diags...)
				tfPermissions[resourceType] = permList
			}
			data.Permissions = tfPermissions
			break
		}
	}

	if !found {
		searchBy := "ID"
		searchValue := data.ID.ValueString()
		if data.ID.IsNull() {
			searchBy = "name"
			searchValue = data.Name.ValueString()
		}
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role with %s '%s' not found", searchBy, searchValue))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
