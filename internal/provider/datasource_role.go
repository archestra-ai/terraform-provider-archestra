package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &RoleDataSource{}
var _ datasource.DataSourceWithConfigure = &RoleDataSource{}

func NewRoleDataSource() datasource.DataSource {
	return &RoleDataSource{}
}

// RoleDataSource defines the data source implementation.
type RoleDataSource struct {
	client *client.ClientWithResponses
}

// RoleDataSourceModel describes the data source data model.
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
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Archestra Role Data Source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Role identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Detailed name of the role",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the role",
			},
			"permissions": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of permissions assigned to the role",
			},
		},
	}
}

func (d *RoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected DataSource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *RoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RoleDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	roleUUID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Role ID",
			fmt.Sprintf("Could not parse role ID: %s", err.Error()),
		)
		return
	}

	res, err := d.client.GetCustomRoleWithResponse(ctx, roleUUID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading role",
			"Could not read role, unexpected error: "+err.Error(),
		)
		return
	}

	if res.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading role",
			fmt.Sprintf("Could not read role, unexpected status: %s", res.HTTPResponse.Status),
		)
		return
	}

	data.Name = types.StringValue(res.JSON200.Name)
	if res.JSON200.Description != nil {
		data.Description = types.StringValue(*res.JSON200.Description)
	} else {
		data.Description = types.StringNull()
	}

	respPermissions := make([]types.String, len(res.JSON200.Permissions))
	for i, p := range res.JSON200.Permissions {
		respPermissions[i] = types.StringValue(p)
	}
	data.Permissions = respPermissions

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
