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
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Permissions []types.String `tfsdk:"permissions"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra role by name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role to look up",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the role",
				Computed:            true,
			},
			"permissions": schema.ListAttribute{
				MarkdownDescription: "List of permissions granted by this role",
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

	apiResp, err := d.client.GetRoleByNameWithResponse(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role: %s", err))
		return
	}

	switch apiResp.StatusCode() {
	case 200:
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", "Empty role body in 200 response")
			return
		}
		data.ID = types.StringValue(apiResp.JSON200.Id)
		data.Name = types.StringValue(apiResp.JSON200.Name)
		if apiResp.JSON200.Description != nil {
			data.Description = types.StringValue(*apiResp.JSON200.Description)
		} else {
			data.Description = types.StringNull()
		}
		data.Permissions = convertStringValues(apiResp.JSON200.Permissions)
	case 404:
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role %s not found", data.Name.ValueString()))
		return
	default:
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Unexpected status %d while reading role", apiResp.StatusCode()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
