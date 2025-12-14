package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &SSOProviderDataSource{}

func NewSSOProviderDataSource() datasource.DataSource {
	return &SSOProviderDataSource{}
}


type SSOProviderDataSource struct {
	client *client.ClientWithResponses
}

type SSOProviderDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Issuer         types.String `tfsdk:"issuer"`
	ProviderID     types.String `tfsdk:"provider_id"`
	Domain         types.String `tfsdk:"domain"`
	OrganizationID types.String `tfsdk:"organization_id"`
	UserID         types.String `tfsdk:"user_id"`
	DomainVerified types.Bool   `tfsdk:"domain_verified"`
}

func (d *SSOProviderDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_provider"
}

func (d *SSOProviderDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an Archestra SSO provider configuration.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "SSO provider identifier",
				Required:            true,
			},
			"issuer": schema.StringAttribute{
				MarkdownDescription: "The issuer identifier for SSO provider",
				Computed:            true,
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "The provider ID (e.g., 'google', 'okta', 'saml')",
				Computed:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain associated with this SSO provider",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Organization ID this SSO provider belongs to",
				Computed:            true,
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "User ID who created this SSO provider",
				Computed:            true,
			},
			"domain_verified": schema.BoolAttribute{
				MarkdownDescription: "Whether domain has been verified",
				Computed:            true,
			},
		},
	}
}

func (d *SSOProviderDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *SSOProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SSOProviderDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API to get SSO provider
	apiResp, err := d.client.GetSsoProviderWithResponse(ctx, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading SSO provider",
			fmt.Sprintf("Could not read SSO provider: %s", err),
		)
		return
	}

	if apiResp.HTTPResponse.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"SSO provider not found",
			fmt.Sprintf("SSO provider with ID %s not found", config.ID.ValueString()),
		)
		return
	}

	if apiResp.HTTPResponse.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error reading SSO provider",
			fmt.Sprintf("Unexpected status code: %d, body: %s", apiResp.HTTPResponse.StatusCode, string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading SSO provider",
			"Empty response body from API",
		)
		return
	}

	
	state := SSOProviderDataSourceModel{
		ID:             types.StringValue(apiResp.JSON200.Id),
		Issuer:         types.StringValue(apiResp.JSON200.Issuer),
		ProviderID:     types.StringValue(""), 
		Domain:         types.StringValue(apiResp.JSON200.Domain),
		OrganizationID: types.StringValue(""), 
		UserID:         types.StringValue(""), 
		DomainVerified: types.BoolValue(apiResp.JSON200.DomainVerified != nil && *apiResp.JSON200.DomainVerified),
	}


	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
