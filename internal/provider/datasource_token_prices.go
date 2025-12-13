package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &TokenPricesDataSource{}

func NewTokenPricesDataSource() datasource.DataSource {
	return &TokenPricesDataSource{}
}

// TokenPricesDataSource defines the data source implementation.
type TokenPricesDataSource struct {
	client *client.ClientWithResponses
}

// TokenPriceModel describes a single token price entry.
type TokenPriceModel struct {
	ID                    types.String `tfsdk:"id"`
	Model                 types.String `tfsdk:"model"`
	PricePerMillionInput  types.String `tfsdk:"price_per_million_input"`
	PricePerMillionOutput types.String `tfsdk:"price_per_million_output"`
}

// TokenPricesDataSourceModel describes the data source data model.
type TokenPricesDataSourceModel struct {
	TokenPrices []TokenPriceModel `tfsdk:"token_prices"`
}

func (d *TokenPricesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token_prices"
}

func (d *TokenPricesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches all token prices from Archestra.",

		Attributes: map[string]schema.Attribute{
			"token_prices": schema.ListNestedAttribute{
				MarkdownDescription: "List of token prices",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Token price identifier",
							Computed:            true,
						},
						"model": schema.StringAttribute{
							MarkdownDescription: "The model name",
							Computed:            true,
						},
						"price_per_million_input": schema.StringAttribute{
							MarkdownDescription: "Price per million input tokens",
							Computed:            true,
						},
						"price_per_million_output": schema.StringAttribute{
							MarkdownDescription: "Price per million output tokens",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *TokenPricesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *TokenPricesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TokenPricesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.GetTokenPricesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read token prices, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	tokenPrices := *apiResp.JSON200
	data.TokenPrices = make([]TokenPriceModel, len(tokenPrices))
	for i, tp := range tokenPrices {
		data.TokenPrices[i] = TokenPriceModel{
			ID:                    types.StringValue(tp.Id.String()),
			Model:                 types.StringValue(tp.Model),
			PricePerMillionInput:  types.StringValue(tp.PricePerMillionInput),
			PricePerMillionOutput: types.StringValue(tp.PricePerMillionOutput),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
