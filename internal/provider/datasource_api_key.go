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

var _ datasource.DataSource = &ApiKeyDataSource{}

func NewApiKeyDataSource() datasource.DataSource {
	return &ApiKeyDataSource{}
}

type ApiKeyDataSource struct {
	client *client.ClientWithResponses
}

type ApiKeyDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	UserID      types.String `tfsdk:"user_id"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Prefix      types.String `tfsdk:"prefix"`
	Start       types.String `tfsdk:"start"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	LastRequest types.String `tfsdk:"last_request"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	Permissions types.Map    `tfsdk:"permissions"`
}

func (d *ApiKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (d *ApiKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lookup an existing `archestra_api_key` by ID. The full `value` is **never** returned — `archestra_api_key` issues it once at Create time only. Use this data source for audit / cross-stack metadata only.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Required: true, MarkdownDescription: "API key identifier."},
			"name":         schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable name."},
			"user_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Owning user UUID."},
			"enabled":      schema.BoolAttribute{Computed: true, MarkdownDescription: "True when the key is active."},
			"prefix":       schema.StringAttribute{Computed: true, MarkdownDescription: "Key prefix (e.g. `arch_`)."},
			"start":        schema.StringAttribute{Computed: true, MarkdownDescription: "First few characters of the key, surfaced in the UI for identification. The full key value is not stored or returned by the backend."},
			"expires_at":   schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 expiry timestamp; null when the key has no expiry."},
			"last_request": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 timestamp of the last request authenticated by this key; null if never used."},
			"created_at":   schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 creation timestamp."},
			"updated_at":   schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 last-modification timestamp."},
			"permissions": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
				MarkdownDescription: "Map of permission domain (e.g. `agent`, `tool`) to allowed actions (`read`, `write`, etc.).",
			},
		},
	}
}

func (d *ApiKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *ApiKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ApiKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.GetApiKeyWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read api key: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("API key %q not found.", data.ID.ValueString()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	row := apiResp.JSON200
	if row.Name != nil {
		data.Name = types.StringValue(*row.Name)
	} else {
		data.Name = types.StringNull()
	}
	data.UserID = types.StringValue(row.UserId)
	if row.Enabled != nil {
		data.Enabled = types.BoolValue(*row.Enabled)
	} else {
		data.Enabled = types.BoolValue(false)
	}
	if row.Prefix != nil {
		data.Prefix = types.StringValue(*row.Prefix)
	} else {
		data.Prefix = types.StringNull()
	}
	if row.Start != nil {
		data.Start = types.StringValue(*row.Start)
	} else {
		data.Start = types.StringNull()
	}
	if row.ExpiresAt != nil {
		data.ExpiresAt = types.StringValue(row.ExpiresAt.Format(time.RFC3339))
	} else {
		data.ExpiresAt = types.StringNull()
	}
	if row.LastRequest != nil {
		data.LastRequest = types.StringValue(row.LastRequest.Format(time.RFC3339))
	} else {
		data.LastRequest = types.StringNull()
	}
	data.CreatedAt = types.StringValue(row.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(row.UpdatedAt.Format(time.RFC3339))

	if row.Permissions != nil {
		entries := make(map[string]attr.Value, len(*row.Permissions))
		for k, v := range *row.Permissions {
			elems := make([]attr.Value, len(v))
			for i, s := range v {
				elems[i] = types.StringValue(s)
			}
			listVal, diags := types.ListValue(types.StringType, elems)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			entries[k] = listVal
		}
		mapVal, diags := types.MapValue(types.ListType{ElemType: types.StringType}, entries)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Permissions = mapVal
	} else {
		data.Permissions = types.MapNull(types.ListType{ElemType: types.StringType})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
