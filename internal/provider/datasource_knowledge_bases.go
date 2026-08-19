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

var _ datasource.DataSource = &KnowledgeBasesDataSource{}

func NewKnowledgeBasesDataSource() datasource.DataSource {
	return &KnowledgeBasesDataSource{}
}

type KnowledgeBasesDataSource struct {
	client *client.ClientWithResponses
}

type KnowledgeBasesDataSourceModel struct {
	Search         types.String `tfsdk:"search"`
	KnowledgeBases types.List   `tfsdk:"knowledge_bases"`
	Total          types.Int64  `tfsdk:"total"`
}

var knowledgeBasesElementType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":          types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
	"status":      types.StringType,
	"created_at":  types.StringType,
	"updated_at":  types.StringType,
}}

func (d *KnowledgeBasesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_bases"
}

func (d *KnowledgeBasesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all knowledge bases in the organization. ~> **Paginates exhaustively** — state size scales linearly with KB count; narrow with `search` on orgs with hundreds of KBs.",
		Attributes: map[string]schema.Attribute{
			"search": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional case-insensitive substring filter on name/description.",
			},
			"knowledge_bases": schema.ListAttribute{
				Computed:            true,
				ElementType:         knowledgeBasesElementType,
				MarkdownDescription: "All knowledge bases matching the filter (or every KB if `search` is omitted).",
			},
			"total": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total count reported by the backend.",
			},
		},
	}
}

func (d *KnowledgeBasesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *KnowledgeBasesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data KnowledgeBasesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	limit := 100
	offset := 0
	params := &client.GetKnowledgeBasesParams{Limit: &limit, Offset: &offset}
	if !data.Search.IsNull() && !data.Search.IsUnknown() {
		s := data.Search.ValueString()
		params.Search = &s
	}

	var collected []attr.Value
	var total int64
	for {
		apiResp, err := d.client.GetKnowledgeBasesWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list knowledge bases: %s", err))
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
			obj, diags := types.ObjectValue(knowledgeBasesElementType.AttrTypes, map[string]attr.Value{
				"id":          types.StringValue(row.Id.String()),
				"name":        types.StringValue(row.Name),
				"description": desc,
				"status":      types.StringValue(row.Status),
				"created_at":  types.StringValue(row.CreatedAt.Format(time.RFC3339)),
				"updated_at":  types.StringValue(row.UpdatedAt.Format(time.RFC3339)),
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

	listValue, diags := types.ListValue(knowledgeBasesElementType, collected)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.KnowledgeBases = listValue
	data.Total = types.Int64Value(total)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
