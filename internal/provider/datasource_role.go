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
	Role           types.String `tfsdk:"role"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Predefined     types.Bool   `tfsdk:"predefined"`
	Permission     types.Map    `tfsdk:"permission"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra role by ID or name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier (base62 for custom roles, or predefined role name: admin, editor, member)",
				Required:            true,
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

	// Call API with union type parameter
	apiResp, err := d.client.GetRoleWithResponse(ctx, makeRoleIdParam(data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	if apiResp.JSON404 != nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role with ID %s not found", data.ID.ValueString()))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Role = types.StringValue(apiResp.JSON200.Role)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)
	if apiResp.JSON200.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}
	data.Permission = convertGetRolePermissionToTerraform(apiResp.JSON200.Permission)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// convertGetRolePermissionToTerraform converts GetRole API permission response to Terraform map
func convertGetRolePermissionToTerraform(permission map[string][]client.GetRole200Permission) types.Map {
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
