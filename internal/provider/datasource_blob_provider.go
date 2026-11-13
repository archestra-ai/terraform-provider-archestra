package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &BlobProviderDataSource{}

func NewBlobProviderDataSource() datasource.DataSource { return &BlobProviderDataSource{} }

// BlobProviderDataSource looks up a blob provider by name, so configuration
// can reference one created in the UI, for example from archestra_cost_export.
type BlobProviderDataSource struct{ client *client.ClientWithResponses }

type BlobProviderDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`
	HasCredentials types.Bool   `tfsdk:"has_credentials"`
}

func (d *BlobProviderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_blob_provider"
}

func (d *BlobProviderDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a blob provider by name (API 1.5.0-rc.30 or later). Fails when no provider, or more than one, has the name.",
		Attributes: map[string]schema.Attribute{
			"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Blob provider name."},
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Blob provider UUID."},
			"type":            schema.StringAttribute{Computed: true, MarkdownDescription: "Provider type: `s3`, `gcs` or `azure_blob`."},
			"has_credentials": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether static credentials are stored for this provider."},
		},
	}
}

func (d *BlobProviderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got %T", req.ProviderData))
		return
	}
	d.client = api
}

func (d *BlobProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BlobProviderDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	raw, found := findBlobProvider(ctx, d.client, func(p blobProviderWire) bool { return p.Name == name }, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.Diagnostics.AddError("Blob Provider Not Found", fmt.Sprintf("No blob provider is named %q.", name))
		return
	}
	var full BlobProviderResourceModel
	flattenBlobProvider(&full, raw, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ID, data.Type, data.HasCredentials = full.ID, full.Type, full.HasCredentials
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
