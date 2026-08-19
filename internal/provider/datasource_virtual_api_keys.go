package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &VirtualApiKeysDataSource{}

func NewVirtualApiKeysDataSource() datasource.DataSource {
	return &VirtualApiKeysDataSource{}
}

type VirtualApiKeysDataSource struct {
	client *client.ClientWithResponses
}

type VirtualApiKeysDataSourceModel struct {
	LlmProviderApiKeyID types.String `tfsdk:"llm_provider_api_key_id"`
	Search              types.String `tfsdk:"search"`
	VirtualApiKeys      types.List   `tfsdk:"virtual_api_keys"`
	Total               types.Int64  `tfsdk:"total"`
}

var virtualApiKeysElementType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                      types.StringType,
	"name":                    types.StringType,
	"scope":                   types.StringType,
	"llm_provider_api_key_id": types.StringType,
	"secret_id":               types.StringType,
	"token_start":             types.StringType,
	"author_id":               types.StringType,
	"author_name":             types.StringType,
	"expires_at":              types.StringType,
	"last_used_at":            types.StringType,
	"created_at":              types.StringType,
}}

func (d *VirtualApiKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_api_keys"
}

func (d *VirtualApiKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every virtual API key visible to the caller; the full token `value` is never returned (use the `archestra_virtual_api_key` resource for create-time issuance). ~> **Paginates exhaustively** — narrow with `llm_provider_api_key_id` or `search` on orgs with many virtual keys.",
		Attributes: map[string]schema.Attribute{
			"llm_provider_api_key_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional filter — only virtual keys issued against this parent provider key.",
			},
			"search": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional case-insensitive substring filter on virtual-key name.",
			},
			"virtual_api_keys": schema.ListAttribute{
				Computed:            true,
				ElementType:         virtualApiKeysElementType,
				MarkdownDescription: "All matching virtual keys (paginated exhaustively).",
			},
			"total": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total count reported by the backend.",
			},
		},
	}
}

func (d *VirtualApiKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VirtualApiKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VirtualApiKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	limit := 100
	offset := 0
	params := &client.GetAllVirtualApiKeysParams{Limit: &limit, Offset: &offset}
	if !data.LlmProviderApiKeyID.IsNull() && !data.LlmProviderApiKeyID.IsUnknown() {
		parsed, err := uuid.Parse(data.LlmProviderApiKeyID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid llm_provider_api_key_id", err.Error())
			return
		}
		params.ChatApiKeyId = &parsed
	}
	if !data.Search.IsNull() && !data.Search.IsUnknown() {
		s := data.Search.ValueString()
		params.Search = &s
	}

	var collected []attr.Value
	var total int64
	for {
		apiResp, err := d.client.GetAllVirtualApiKeysWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list virtual API keys: %s", err))
			return
		}
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}
		total = int64(apiResp.JSON200.Pagination.Total)
		for i := range apiResp.JSON200.Data {
			row := &apiResp.JSON200.Data[i]
			obj, diags := types.ObjectValue(virtualApiKeysElementType.AttrTypes, map[string]attr.Value{
				"id":                      types.StringValue(row.Id.String()),
				"name":                    types.StringValue(row.Name),
				"scope":                   types.StringValue(string(row.Scope)),
				"llm_provider_api_key_id": types.StringValue(row.ChatApiKeyId.String()),
				"secret_id":               types.StringValue(row.SecretId.String()),
				"token_start":             types.StringValue(row.TokenStart),
				"author_id":               optionalString(row.AuthorId),
				"author_name":             optionalString(row.AuthorName),
				"expires_at":              optionalTime(row.ExpiresAt),
				"last_used_at":            optionalTime(row.LastUsedAt),
				"created_at":              types.StringValue(row.CreatedAt.Format(time.RFC3339)),
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

	listValue, diags := types.ListValue(virtualApiKeysElementType, collected)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.VirtualApiKeys = listValue
	data.Total = types.Int64Value(total)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// optionalString turns a *string into a types.String null when nil.
// Defined here (not in a shared file) because the existing
// optionalStringFromAPI takes a pointer-to-target, which doesn't
// compose nicely with map literal construction.
func optionalString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

// optionalTime mirrors optionalString for nullable timestamps,
// formatting as RFC 3339 when present.
func optionalTime(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
