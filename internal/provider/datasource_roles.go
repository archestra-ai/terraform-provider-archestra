package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &RolesDataSource{}

func NewRolesDataSource() datasource.DataSource {
	return &RolesDataSource{}
}

type RolesDataSource struct {
	client *client.ClientWithResponses
}

type RolesDataSourceModel struct {
	Name  types.String `tfsdk:"name"`
	Roles types.List   `tfsdk:"roles"`
	Total types.Int64  `tfsdk:"total"`
}

var rolesElementType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":          types.StringType,
	"role":        types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
	"predefined":  types.BoolType,
	"created_at":  types.StringType,
	"updated_at":  types.StringType,
}}

func (d *RolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_roles"
}

func (d *RolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists roles in the organization (predefined + custom). ~> **Paginates exhaustively** — narrow with `name` on orgs with many custom roles.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional case-insensitive substring filter on role name.",
			},
			"roles": schema.ListAttribute{
				Computed:            true,
				ElementType:         rolesElementType,
				MarkdownDescription: "Roles matching the filter, including both predefined (admin, member, owner, …) and custom rows.",
			},
			"total": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total count reported by the backend.",
			},
		},
	}
}

func (d *RolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *RolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RolesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	limit := 100
	offset := 0
	params := &client.GetRolesParams{Limit: &limit, Offset: &offset}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		s := data.Name.ValueString()
		params.Name = &s
	}

	var collected []attr.Value
	var total int64
	for {
		apiResp, err := d.client.GetRolesWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list roles: %s", err))
			return
		}
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}
		total = int64(apiResp.JSON200.Pagination.Total)
		for i := range apiResp.JSON200.Data {
			row := &apiResp.JSON200.Data[i]
			desc := types.StringNull()
			if row.Description != nil {
				desc = types.StringValue(*row.Description)
			}
			updated := types.StringNull()
			if row.UpdatedAt != nil {
				updated = types.StringValue(row.UpdatedAt.Format(time.RFC3339))
			}
			obj, diags := types.ObjectValue(rolesElementType.AttrTypes, map[string]attr.Value{
				"id":          types.StringValue(row.Id),
				"role":        types.StringValue(row.Role),
				"name":        types.StringValue(row.Name),
				"description": desc,
				"predefined":  types.BoolValue(row.Predefined),
				"created_at":  types.StringValue(row.CreatedAt.Format(time.RFC3339)),
				"updated_at":  updated,
			})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			collected = append(collected, obj)
		}
		if !apiResp.JSON200.Pagination.HasNext {
			break
		}
		offset += limit
		params.Offset = &offset
	}

	listValue, diags := types.ListValue(rolesElementType, collected)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Roles = listValue
	data.Total = types.Int64Value(total)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
