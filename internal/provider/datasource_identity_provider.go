package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &IdentityProviderDataSource{}

func NewIdentityProviderDataSource() datasource.DataSource {
	return &IdentityProviderDataSource{}
}

type IdentityProviderDataSource struct {
	client *client.ClientWithResponses
}

// IdentityProviderDataSourceModel exposes the top-level IdP scalars
// useful for cross-stack discovery (e.g. binding policies or members to
// an existing IdP by ID). The deeply-nested OIDC / SAML credential
// payloads are deliberately omitted — the GET endpoint returns
// `clientSecret` and `decryptionPvk` in plaintext, which would land in
// state files and trip every security scanner. Users who need
// credentials should manage the IdP via `archestra_identity_provider`
// instead.
type IdentityProviderDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProviderID       types.String `tfsdk:"provider_id"`
	Domain           types.String `tfsdk:"domain"`
	DomainVerified   types.Bool   `tfsdk:"domain_verified"`
	Issuer           types.String `tfsdk:"issuer"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	HasOidcConfig    types.Bool   `tfsdk:"has_oidc_config"`
	HasSamlConfig    types.Bool   `tfsdk:"has_saml_config"`
	OidcDiscoveryURL types.String `tfsdk:"oidc_discovery_endpoint"`
	OidcClientID     types.String `tfsdk:"oidc_client_id"`
	SamlEntryPoint   types.String `tfsdk:"saml_entry_point"`
	SamlCallbackURL  types.String `tfsdk:"saml_callback_url"`
}

func (d *IdentityProviderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_provider"
}

func (d *IdentityProviderDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lookup an existing `archestra_identity_provider` by ID. Surfaces only the non-credential scalar fields — `clientSecret`, `decryptionPvk`, and similar secret fields the GET endpoint returns in plaintext are deliberately not exposed (managing them belongs in `archestra_identity_provider`, not a data source).",
		Attributes: map[string]schema.Attribute{
			"id":                      schema.StringAttribute{Required: true, MarkdownDescription: "Identity provider identifier."},
			"provider_id":             schema.StringAttribute{Computed: true, MarkdownDescription: "Backend `providerId` (matches `id` for most provisioning paths)."},
			"domain":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Email domain mapped to this IdP."},
			"domain_verified":         schema.BoolAttribute{Computed: true, MarkdownDescription: "True when the domain has been verified by the backend."},
			"issuer":                  schema.StringAttribute{Computed: true, MarkdownDescription: "IdP-level issuer URL."},
			"organization_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Owning org UUID; null on system-level IdPs."},
			"has_oidc_config":         schema.BoolAttribute{Computed: true, MarkdownDescription: "True when the IdP is configured with an OIDC block."},
			"has_saml_config":         schema.BoolAttribute{Computed: true, MarkdownDescription: "True when the IdP is configured with a SAML block."},
			"oidc_discovery_endpoint": schema.StringAttribute{Computed: true, MarkdownDescription: "OIDC discovery URL (when `has_oidc_config = true`); null otherwise."},
			"oidc_client_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "OIDC client ID (when `has_oidc_config = true`); null otherwise. `client_secret` is intentionally not exposed."},
			"saml_entry_point":        schema.StringAttribute{Computed: true, MarkdownDescription: "SAML SSO entry-point URL (when `has_saml_config = true`); null otherwise."},
			"saml_callback_url":       schema.StringAttribute{Computed: true, MarkdownDescription: "SAML callback URL (when `has_saml_config = true`); null otherwise."},
		},
	}
}

func (d *IdentityProviderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IdentityProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IdentityProviderDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.GetIdentityProviderWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read identity provider: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Identity provider %q not found.", data.ID.ValueString()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	row := apiResp.JSON200
	data.ProviderID = types.StringValue(row.ProviderId)
	data.Domain = types.StringValue(row.Domain)
	if row.DomainVerified != nil {
		data.DomainVerified = types.BoolValue(*row.DomainVerified)
	} else {
		data.DomainVerified = types.BoolValue(false)
	}
	data.Issuer = types.StringValue(row.Issuer)
	if row.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*row.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}
	data.HasOidcConfig = types.BoolValue(row.OidcConfig != nil)
	data.HasSamlConfig = types.BoolValue(row.SamlConfig != nil)
	if row.OidcConfig != nil {
		data.OidcDiscoveryURL = types.StringValue(row.OidcConfig.DiscoveryEndpoint)
		data.OidcClientID = types.StringValue(row.OidcConfig.ClientId)
	} else {
		data.OidcDiscoveryURL = types.StringNull()
		data.OidcClientID = types.StringNull()
	}
	if row.SamlConfig != nil {
		data.SamlEntryPoint = types.StringValue(row.SamlConfig.EntryPoint)
		data.SamlCallbackURL = types.StringValue(row.SamlConfig.CallbackUrl)
	} else {
		data.SamlEntryPoint = types.StringNull()
		data.SamlCallbackURL = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
