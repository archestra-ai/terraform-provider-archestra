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

var _ datasource.DataSource = &OrganizationMembersDataSource{}

func NewOrganizationMembersDataSource() datasource.DataSource {
	return &OrganizationMembersDataSource{}
}

type OrganizationMembersDataSource struct {
	client *client.ClientWithResponses
}

type OrganizationMembersDataSourceModel struct {
	Name    types.String `tfsdk:"name"`
	Role    types.String `tfsdk:"role"`
	Members types.List   `tfsdk:"members"`
	Total   types.Int64  `tfsdk:"total"`
}

var organizationMembersElementType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":         types.StringType,
	"user_id":    types.StringType,
	"name":       types.StringType,
	"email":      types.StringType,
	"image":      types.StringType,
	"role":       types.StringType,
	"created_at": types.StringType,
}}

func (d *OrganizationMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_members"
}

func (d *OrganizationMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists members of the organization; useful for cross-referencing user IDs against IdP-synced groups or auditing role assignments. ~> **Paginates exhaustively** — narrow with `name` or `role` on orgs with thousands of members.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional case-insensitive substring filter on name or email.",
			},
			"role": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional exact-match filter on role name.",
			},
			"members": schema.ListAttribute{
				Computed:            true,
				ElementType:         organizationMembersElementType,
				MarkdownDescription: "All organization members matching the filters (paginated exhaustively).",
			},
			"total": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total count reported by the backend.",
			},
		},
	}
}

func (d *OrganizationMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationMembersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	limit := 100
	offset := 0
	params := &client.GetMembersParams{Limit: &limit, Offset: &offset}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		s := data.Name.ValueString()
		params.Name = &s
	}
	if !data.Role.IsNull() && !data.Role.IsUnknown() {
		s := data.Role.ValueString()
		params.Role = &s
	}

	var collected []attr.Value
	var total int64
	for {
		apiResp, err := d.client.GetMembersWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list members: %s", err))
			return
		}
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}
		total = int64(apiResp.JSON200.Pagination.Total)
		for i := range apiResp.JSON200.Data {
			row := &apiResp.JSON200.Data[i]
			obj, diags := types.ObjectValue(organizationMembersElementType.AttrTypes, map[string]attr.Value{
				"id":         types.StringValue(row.Id),
				"user_id":    types.StringValue(row.UserId),
				"name":       optionalString(row.Name),
				"email":      types.StringValue(row.Email),
				"image":      optionalString(row.Image),
				"role":       types.StringValue(row.Role),
				"created_at": types.StringValue(row.CreatedAt.Format(time.RFC3339)),
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

	listValue, diags := types.ListValue(organizationMembersElementType, collected)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Members = listValue
	data.Total = types.Int64Value(total)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
