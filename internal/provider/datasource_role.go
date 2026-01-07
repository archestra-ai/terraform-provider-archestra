package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Permissions    types.Map    `tfsdk:"permissions"`
	Predefined     types.Bool   `tfsdk:"predefined"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra role by ID or name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role",
				Optional:            true,
				Computed:            true,
			},
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Map of resource names to list of allowed actions",
				Computed:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
			},
			"predefined": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a system-defined role",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID this role belongs to",
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
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
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

	// Validate that at least one of id or name is provided
	if data.ID.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either 'id' or 'name' must be specified",
		)
		return
	}

	// List all roles and filter
	rolesResp, err := d.client.GetRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list roles, got error: %s", err))
		return
	}

	if rolesResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", rolesResp.StatusCode()))
		return
	}

	// Find role by ID or name
	targetID := data.ID.ValueString()
	targetName := data.Name.ValueString()

	for _, role := range *rolesResp.JSON200 {
		match := false
		if !data.ID.IsNull() && role.Id == targetID {
			match = true
		} else if !data.Name.IsNull() && role.Name == targetName {
			match = true
		}

		if match {
			// Convert permissions
			permissions := make(map[string][]string)
			for k, v := range role.Permission {
				strSlice := make([]string, len(v))
				for i, perm := range v {
					strSlice[i] = string(perm)
				}
				permissions[k] = strSlice
			}

			d.updateDataFromResponse(&data, role.Id, role.Name,
				permissions, role.Predefined, role.OrganizationId, &resp.Diagnostics)

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	if !data.ID.IsNull() {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role with ID '%s' not found", targetID))
	} else {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role with name '%s' not found", targetName))
	}
}

// Helper to update data from response
func (d *RoleDataSource) updateDataFromResponse(data *RoleDataSourceModel, id, name string, permissions map[string][]string, predefined bool, orgId *string, diagsResult *diag.Diagnostics) {
	data.ID = types.StringValue(id)
	data.Name = types.StringValue(name)
	data.Predefined = types.BoolValue(predefined)

	if orgId != nil {
		data.OrganizationID = types.StringValue(*orgId)
	} else {
		data.OrganizationID = types.StringNull()
	}

	// Build the permissions map
	tfPermMap := make(map[string]attr.Value)
	for k, v := range permissions {
		// Sort for consistent ordering
		sorted := make([]string, len(v))
		copy(sorted, v)
		sort.Strings(sorted)

		listVals := make([]attr.Value, len(sorted))
		for i, s := range sorted {
			listVals[i] = types.StringValue(s)
		}
		listVal, listDiags := types.ListValue(types.StringType, listVals)
		diagsResult.Append(listDiags...)
		tfPermMap[k] = listVal
	}

	mapVal, mapDiags := types.MapValue(types.ListType{ElemType: types.StringType}, tfPermMap)
	diagsResult.Append(mapDiags...)
	data.Permissions = mapVal
}
