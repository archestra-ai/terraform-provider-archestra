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

var _ datasource.DataSource = &MCPCatalogLabelKeysDataSource{}

func NewMCPCatalogLabelKeysDataSource() datasource.DataSource {
	return &MCPCatalogLabelKeysDataSource{}
}

type MCPCatalogLabelKeysDataSource struct {
	client *client.ClientWithResponses
}

type MCPCatalogLabelKeysDataSourceModel struct {
	Keys types.List `tfsdk:"keys"`
}

func (d *MCPCatalogLabelKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_catalog_label_keys"
}

func (d *MCPCatalogLabelKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Distinct label keys across all MCP registry catalog items. Useful for validating label-based catalog-item references before applying.",
		Attributes: map[string]schema.Attribute{
			"keys": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Distinct label keys present on at least one catalog item.",
			},
		},
	}
}

func (d *MCPCatalogLabelKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *MCPCatalogLabelKeysDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.GetInternalMcpCatalogLabelKeysWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list catalog label keys: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	keys := make([]attr.Value, len(*apiResp.JSON200))
	for i, k := range *apiResp.JSON200 {
		keys[i] = types.StringValue(k)
	}
	listVal, diags := types.ListValue(types.StringType, keys)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data := MCPCatalogLabelKeysDataSourceModel{Keys: listVal}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
