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
	ID          types.String              `tfsdk:"id"`
	Name        types.String              `tfsdk:"name"`
	Description types.String              `tfsdk:"description"`
	Permissions map[string][]types.String `tfsdk:"permissions"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a role by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Role description",
				Computed:            true,
			},
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Permissions associated with the role",
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

	roleID := data.ID.ValueString()

	roleResp, err := d.client.GetRoleWithResponse(ctx, roleID)

	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	if roleResp.JSON404 != nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role with ID %s not found", roleID))
		return
	}

	if roleResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", roleResp.StatusCode()))
		return
	}

	role := roleResp.JSON200

	data.Name = types.StringValue(role.Name)
	if role.Role != "" {
		data.Description = types.StringValue(role.Role)
	} else {
		data.Description = types.StringNull()
	}

	if role.Permission != nil {
		perms := make(map[string][]types.String)
		for k, v := range role.Permission {
			var permList []types.String
			for _, p := range v {
				permList = append(permList, types.StringValue(string(p)))
			}
			perms[k] = permList
		}
		data.Permissions = perms
	} else {
		data.Permissions = nil
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
