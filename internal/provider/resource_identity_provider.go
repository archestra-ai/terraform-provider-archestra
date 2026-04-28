package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &IdentityProviderResource{}
var _ resource.ResourceWithImportState = &IdentityProviderResource{}

func NewIdentityProviderResource() resource.Resource {
	return &IdentityProviderResource{}
}

// IdentityProviderResource manages identity providers (OIDC or SAML).
type IdentityProviderResource struct {
	client *client.ClientWithResponses
}

type IdentityProviderResourceModel struct {
	ID             types.String         `tfsdk:"id"`
	ProviderID     types.String         `tfsdk:"provider_id"`
	Domain         types.String         `tfsdk:"domain"`
	DomainVerified types.Bool           `tfsdk:"domain_verified"`
	Issuer         types.String         `tfsdk:"issuer"`
	OidcConfig     *OidcConfigModel     `tfsdk:"oidc_config"`
	SamlConfig     *SamlConfigModel     `tfsdk:"saml_config"`
	RoleMapping    *RoleMappingModel    `tfsdk:"role_mapping"`
	TeamSyncConfig *TeamSyncConfigModel `tfsdk:"team_sync_config"`
	UserID         types.String         `tfsdk:"user_id"`
	OrganizationID types.String         `tfsdk:"organization_id"`
}

type OidcConfigModel struct {
	Issuer                       types.String                       `tfsdk:"issuer"`
	DiscoveryEndpoint            types.String                       `tfsdk:"discovery_endpoint"`
	ClientID                     types.String                       `tfsdk:"client_id"`
	ClientSecret                 types.String                       `tfsdk:"client_secret"`
	AuthorizationEndpoint        types.String                       `tfsdk:"authorization_endpoint"`
	TokenEndpoint                types.String                       `tfsdk:"token_endpoint"`
	UserInfoEndpoint             types.String                       `tfsdk:"user_info_endpoint"`
	JwksEndpoint                 types.String                       `tfsdk:"jwks_endpoint"`
	Scopes                       []types.String                     `tfsdk:"scopes"`
	Pkce                         types.Bool                         `tfsdk:"pkce"`
	OverrideUserInfo             types.Bool                         `tfsdk:"override_user_info"`
	SkipDiscovery                types.Bool                         `tfsdk:"skip_discovery"`
	EnableRpInitiatedLogout      types.Bool                         `tfsdk:"enable_rp_initiated_logout"`
	Hd                           types.String                       `tfsdk:"hd"`
	TokenEndpointAuthentication  types.String                       `tfsdk:"token_endpoint_authentication"`
	Mapping                      *OidcMappingModel                  `tfsdk:"mapping"`
	EnterpriseManagedCredentials *EnterpriseManagedCredentialsModel `tfsdk:"enterprise_managed_credentials"`
}

type EnterpriseManagedCredentialsModel struct {
	ExchangeStrategy            types.String `tfsdk:"exchange_strategy"`
	ClientID                    types.String `tfsdk:"client_id"`
	ClientSecret                types.String `tfsdk:"client_secret"`
	TokenEndpoint               types.String `tfsdk:"token_endpoint"`
	TokenEndpointAuthentication types.String `tfsdk:"token_endpoint_authentication"`
	PrivateKeyPem               types.String `tfsdk:"private_key_pem"`
	PrivateKeyID                types.String `tfsdk:"private_key_id"`
	ClientAssertionAudience     types.String `tfsdk:"client_assertion_audience"`
	SubjectTokenType            types.String `tfsdk:"subject_token_type"`
}

type OidcMappingModel struct {
	Email         types.String `tfsdk:"email"`
	EmailVerified types.String `tfsdk:"email_verified"`
	ExtraFields   types.Map    `tfsdk:"extra_fields"`
	ID            types.String `tfsdk:"id"`
	Image         types.String `tfsdk:"image"`
	Name          types.String `tfsdk:"name"`
}

type SamlConfigModel struct {
	Issuer               types.String         `tfsdk:"issuer"`
	EntryPoint           types.String         `tfsdk:"entry_point"`
	CallbackURL          types.String         `tfsdk:"callback_url"`
	Cert                 types.String         `tfsdk:"cert"`
	Audience             types.String         `tfsdk:"audience"`
	DigestAlgorithm      types.String         `tfsdk:"digest_algorithm"`
	IdentifierFormat     types.String         `tfsdk:"identifier_format"`
	DecryptionPvk        types.String         `tfsdk:"decryption_pvk"`
	PrivateKey           types.String         `tfsdk:"private_key"`
	SignatureAlgorithm   types.String         `tfsdk:"signature_algorithm"`
	WantAssertionsSigned types.Bool           `tfsdk:"want_assertions_signed"`
	IdpMetadata          *SamlIdpMetadata     `tfsdk:"idp_metadata"`
	SpMetadata           *SamlSpMetadata      `tfsdk:"sp_metadata"`
	Mapping              *SamlMappingModel    `tfsdk:"mapping"`
	AdditionalParams     jsontypes.Normalized `tfsdk:"additional_params"`
}

type SamlIdpMetadata struct {
	Cert                 types.String `tfsdk:"cert"`
	EncPrivateKey        types.String `tfsdk:"enc_private_key"`
	EncPrivateKeyPass    types.String `tfsdk:"enc_private_key_pass"`
	EntityID             types.String `tfsdk:"entity_id"`
	EntityURL            types.String `tfsdk:"entity_url"`
	IsAssertionEncrypted types.Bool   `tfsdk:"is_assertion_encrypted"`
	Metadata             types.String `tfsdk:"metadata"`
	PrivateKey           types.String `tfsdk:"private_key"`
	PrivateKeyPass       types.String `tfsdk:"private_key_pass"`
	RedirectURL          types.String `tfsdk:"redirect_url"`
	SingleSignOnService  []SsoService `tfsdk:"single_sign_on_service"`
}

type SsoService struct {
	Binding  types.String `tfsdk:"binding"`
	Location types.String `tfsdk:"location"`
}

type SamlSpMetadata struct {
	Binding              types.String `tfsdk:"binding"`
	EncPrivateKey        types.String `tfsdk:"enc_private_key"`
	EncPrivateKeyPass    types.String `tfsdk:"enc_private_key_pass"`
	EntityID             types.String `tfsdk:"entity_id"`
	IsAssertionEncrypted types.Bool   `tfsdk:"is_assertion_encrypted"`
	Metadata             types.String `tfsdk:"metadata"`
	PrivateKey           types.String `tfsdk:"private_key"`
	PrivateKeyPass       types.String `tfsdk:"private_key_pass"`
}

type SamlMappingModel struct {
	Email         types.String `tfsdk:"email"`
	EmailVerified types.String `tfsdk:"email_verified"`
	ExtraFields   types.Map    `tfsdk:"extra_fields"`
	FirstName     types.String `tfsdk:"first_name"`
	ID            types.String `tfsdk:"id"`
	LastName      types.String `tfsdk:"last_name"`
	Name          types.String `tfsdk:"name"`
}

type RoleMappingModel struct {
	DefaultRole  types.String    `tfsdk:"default_role"`
	SkipRoleSync types.Bool      `tfsdk:"skip_role_sync"`
	StrictMode   types.Bool      `tfsdk:"strict_mode"`
	Rules        []RoleRuleModel `tfsdk:"rules"`
}

type RoleRuleModel struct {
	Expression types.String `tfsdk:"expression"`
	Role       types.String `tfsdk:"role"`
}

type TeamSyncConfigModel struct {
	Enabled          types.Bool   `tfsdk:"enabled"`
	GroupsExpression types.String `tfsdk:"groups_expression"`
}

func (r *IdentityProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_provider"
}

func (r *IdentityProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages Archestra identity providers (OIDC or SAML). Exactly one of oidc_config or saml_config must be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "identity provider identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"provider_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique provider identifier (e.g. Okta, Google, Keycloak).",
			},
			"domain": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization domain associated with this provider.",
			},
			"issuer": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Issuer for this provider (OIDC issuer URL or SAML entity ID).",
			},
			"domain_verified": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the domain has been verified.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID for the provider.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User who created the provider.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"oidc_config": schema.SingleNestedBlock{
				MarkdownDescription: "OIDC configuration (cannot be set with saml_config).",
				Attributes: map[string]schema.Attribute{
					"issuer":                        schema.StringAttribute{Optional: true, MarkdownDescription: "OIDC issuer URL."},
					"discovery_endpoint":            schema.StringAttribute{Optional: true, MarkdownDescription: "Discovery endpoint (.well-known)."},
					"client_id":                     schema.StringAttribute{Optional: true, MarkdownDescription: "OIDC client ID."},
					"client_secret":                 schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "OIDC client secret."},
					"authorization_endpoint":        schema.StringAttribute{Optional: true, MarkdownDescription: "Override authorization endpoint."},
					"token_endpoint":                schema.StringAttribute{Optional: true, MarkdownDescription: "Override token endpoint."},
					"user_info_endpoint":            schema.StringAttribute{Optional: true, MarkdownDescription: "Override user info endpoint."},
					"jwks_endpoint":                 schema.StringAttribute{Optional: true, MarkdownDescription: "Override JWKS endpoint."},
					"scopes":                        schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "OAuth scopes to request."},
					"pkce":                          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable PKCE."},
					"override_user_info":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Use token claims instead of userinfo when true."},
					"skip_discovery":                schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Skip OIDC discovery endpoint validation."},
					"enable_rp_initiated_logout":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable RP-initiated logout."},
					"hd":                            schema.StringAttribute{Optional: true, MarkdownDescription: "Google Hosted Domain restriction (e.g., `example.com`). Only allows users from this domain."},
					"token_endpoint_authentication": schema.StringAttribute{Optional: true, MarkdownDescription: "Token endpoint auth method (client_secret_basic or client_secret_post)."},
				},
				Blocks: map[string]schema.Block{
					"mapping": schema.SingleNestedBlock{
						MarkdownDescription: "Attribute mapping for user fields. The `id` field is required by the better-auth OIDC library; defaults to `\"sub\"` if unset.",
						Attributes: map[string]schema.Attribute{
							"email":          schema.StringAttribute{Optional: true},
							"email_verified": schema.StringAttribute{Optional: true},
							"extra_fields":   schema.MapAttribute{Optional: true, ElementType: types.StringType},
							"id": schema.StringAttribute{
								Optional:            true,
								Computed:            true,
								Default:             stringdefault.StaticString("sub"),
								MarkdownDescription: "OIDC claim that maps to the user identity. Defaults to `\"sub\"` (the better-auth OIDC library treats this as required and the backend silently fills it in).",
							},
							"image": schema.StringAttribute{Optional: true},
							"name":  schema.StringAttribute{Optional: true},
						},
					},
					"enterprise_managed_credentials": schema.SingleNestedBlock{
						MarkdownDescription: "Enterprise-managed credentials for token exchange flows.",
						Attributes: map[string]schema.Attribute{
							"exchange_strategy":             schema.StringAttribute{Optional: true, MarkdownDescription: "Downstream token exchange strategy. One of `rfc8693` (generic OIDC), `okta_managed` (Okta-managed credentials), `entra_obo` (Microsoft Entra OBO)."},
							"client_id":                     schema.StringAttribute{Optional: true, MarkdownDescription: "Client ID for enterprise-managed credentials."},
							"client_secret":                 schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Client secret for enterprise-managed credentials."},
							"token_endpoint":                schema.StringAttribute{Optional: true, MarkdownDescription: "Token endpoint URL."},
							"token_endpoint_authentication": schema.StringAttribute{Optional: true, MarkdownDescription: "Token endpoint auth method: client_secret_post, client_secret_basic, or private_key_jwt."},
							"private_key_pem":               schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "PEM-encoded private key for private_key_jwt authentication."},
							"private_key_id":                schema.StringAttribute{Optional: true, MarkdownDescription: "Key ID for the private key."},
							"client_assertion_audience":     schema.StringAttribute{Optional: true, MarkdownDescription: "Audience for client assertion JWT."},
							"subject_token_type":            schema.StringAttribute{Optional: true, MarkdownDescription: "Subject token type for token exchange."},
						},
					},
				},
			},
			"saml_config": schema.SingleNestedBlock{
				MarkdownDescription: "SAML configuration (cannot be set with oidc_config).",
				Attributes: map[string]schema.Attribute{
					"issuer":                 schema.StringAttribute{Optional: true, MarkdownDescription: "SAML issuer/entity ID."},
					"entry_point":            schema.StringAttribute{Optional: true, MarkdownDescription: "IdP SSO entry point URL."},
					"callback_url":           schema.StringAttribute{Optional: true, MarkdownDescription: "ACS callback URL."},
					"cert":                   schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "IdP certificate (X.509)."},
					"audience":               schema.StringAttribute{Optional: true},
					"digest_algorithm":       schema.StringAttribute{Optional: true},
					"identifier_format":      schema.StringAttribute{Optional: true},
					"decryption_pvk":         schema.StringAttribute{Optional: true, Sensitive: true},
					"private_key":            schema.StringAttribute{Optional: true, Sensitive: true},
					"signature_algorithm":    schema.StringAttribute{Optional: true},
					"want_assertions_signed": schema.BoolAttribute{Optional: true},
					"additional_params": schema.StringAttribute{
						Optional:            true,
						CustomType:          jsontypes.NormalizedType{},
						MarkdownDescription: "JSON-encoded map of extra SAML request parameters forwarded to the IdP (booleans, numbers, and nested structures preserved). Must be a JSON object; use `jsonencode({...})`.",
						Validators: []validator.String{
							jsonObjectValidator(),
						},
					},
				},
				Blocks: map[string]schema.Block{
					"idp_metadata": schema.SingleNestedBlock{
						MarkdownDescription: "IdP metadata details.",
						Attributes: map[string]schema.Attribute{
							"cert":                   schema.StringAttribute{Optional: true, Sensitive: true},
							"enc_private_key":        schema.StringAttribute{Optional: true, Sensitive: true},
							"enc_private_key_pass":   schema.StringAttribute{Optional: true, Sensitive: true},
							"entity_id":              schema.StringAttribute{Optional: true},
							"entity_url":             schema.StringAttribute{Optional: true},
							"is_assertion_encrypted": schema.BoolAttribute{Optional: true},
							"metadata":               schema.StringAttribute{Optional: true},
							"private_key":            schema.StringAttribute{Optional: true, Sensitive: true},
							"private_key_pass":       schema.StringAttribute{Optional: true, Sensitive: true},
							"redirect_url":           schema.StringAttribute{Optional: true},
						},
						Blocks: map[string]schema.Block{
							"single_sign_on_service": schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										"binding":  schema.StringAttribute{Required: true},
										"location": schema.StringAttribute{Required: true},
									},
								},
							},
						},
					},
					"sp_metadata": schema.SingleNestedBlock{
						MarkdownDescription: "SP metadata details.",
						Attributes: map[string]schema.Attribute{
							"binding":                schema.StringAttribute{Optional: true},
							"enc_private_key":        schema.StringAttribute{Optional: true, Sensitive: true},
							"enc_private_key_pass":   schema.StringAttribute{Optional: true, Sensitive: true},
							"entity_id":              schema.StringAttribute{Optional: true},
							"is_assertion_encrypted": schema.BoolAttribute{Optional: true},
							"metadata":               schema.StringAttribute{Optional: true},
							"private_key":            schema.StringAttribute{Optional: true, Sensitive: true},
							"private_key_pass":       schema.StringAttribute{Optional: true, Sensitive: true},
						},
					},
					"mapping": schema.SingleNestedBlock{
						MarkdownDescription: "Attribute mapping for user fields. The `id` field is required by the better-auth SAML library; defaults to `\"sub\"` if unset.",
						Attributes: map[string]schema.Attribute{
							"email":          schema.StringAttribute{Optional: true},
							"email_verified": schema.StringAttribute{Optional: true},
							"extra_fields":   schema.MapAttribute{Optional: true, ElementType: types.StringType},
							"first_name":     schema.StringAttribute{Optional: true},
							"id": schema.StringAttribute{
								Optional:            true,
								Computed:            true,
								Default:             stringdefault.StaticString("sub"),
								MarkdownDescription: "SAML attribute that maps to the user identity. Defaults to `\"sub\"` (the better-auth SAML library treats this as required).",
							},
							"last_name": schema.StringAttribute{Optional: true},
							"name":      schema.StringAttribute{Optional: true},
						},
					},
				},
			},
			"role_mapping": schema.SingleNestedBlock{
				MarkdownDescription: "Optional role mapping rules using Handlebars expressions.",
				Attributes: map[string]schema.Attribute{
					"default_role":   schema.StringAttribute{Optional: true},
					"skip_role_sync": schema.BoolAttribute{Optional: true},
					"strict_mode":    schema.BoolAttribute{Optional: true},
					"rules": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"expression": schema.StringAttribute{Required: true},
							"role":       schema.StringAttribute{Required: true},
						}},
					},
				},
			},
			"team_sync_config": schema.SingleNestedBlock{
				MarkdownDescription: "Optional team sync configuration for group extraction.",
				Attributes: map[string]schema.Attribute{
					"enabled":           schema.BoolAttribute{Optional: true},
					"groups_expression": schema.StringAttribute{Optional: true},
				},
			},
		},
	}
}

func (r *IdentityProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	apiClient, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = apiClient
}

func (r *IdentityProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IdentityProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateIdentityProviderConfigChoice(data.OidcConfig, data.SamlConfig); err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}

	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, identityProviderAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_identity_provider Create", patch, identityProviderAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal merge patch: %s", err))
		return
	}

	apiResp, err := r.client.CreateIdentityProviderWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create identity provider: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	state := data
	state.ID = types.StringValue(apiResp.JSON200.Id)
	state.ProviderID = types.StringValue(apiResp.JSON200.ProviderId)
	state.Domain = types.StringValue(apiResp.JSON200.Domain)
	state.Issuer = types.StringValue(apiResp.JSON200.Issuer)

	if apiResp.JSON200.DomainVerified != nil {
		state.DomainVerified = types.BoolValue(*apiResp.JSON200.DomainVerified)
	}
	if apiResp.JSON200.OrganizationId != nil {
		state.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	}
	if apiResp.JSON200.UserId != nil {
		state.UserID = types.StringValue(*apiResp.JSON200.UserId)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IdentityProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IdentityProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetIdentityProviderWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read identity provider: %s", err))
		return
	}

	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	// role_mapping and team_sync_config are pulled only when the user
	// already manages them. The backend's zod for both is `.optional()`
	// (not `.nullable()`), so OmitOnNull on the send side means dropping
	// the block from HCL is a no-op server-side — pulling them back in
	// would surface a phantom "remove this block" plan after refresh.
	isImport := state.OidcConfig == nil && state.SamlConfig == nil
	populateRoleMapping := isImport || state.RoleMapping != nil
	populateTeamSync := isImport || state.TeamSyncConfig != nil

	newState := state
	if err := mapIdentityProviderResponse(apiResp.JSON200, &newState, populateRoleMapping, populateTeamSync); err != nil {
		resp.Diagnostics.AddError("Failed to map identity provider response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *IdentityProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan IdentityProviderResourceModel
	var state IdentityProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateIdentityProviderConfigChoice(plan.OidcConfig, plan.SamlConfig); err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, identityProviderAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_identity_provider Update", patch, identityProviderAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal merge patch: %s", err))
		return
	}

	apiResp, err := r.client.UpdateIdentityProviderWithBodyWithResponse(ctx, state.ID.ValueString(), "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update identity provider: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	newState := plan
	if err := mapIdentityProviderResponse(apiResp.JSON200, &newState, plan.RoleMapping != nil, plan.TeamSyncConfig != nil); err != nil {
		resp.Diagnostics.AddError("Failed to map identity provider response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *IdentityProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state IdentityProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteIdentityProviderWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete identity provider: %s", err))
		return
	}

	if apiResp.StatusCode() != 200 && apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Delete returned status %d", apiResp.StatusCode()))
	}
}

func (r *IdentityProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helpers

func isOidcConfigSet(cfg *OidcConfigModel) bool {
	if cfg == nil {
		return false
	}
	return !cfg.Issuer.IsNull() ||
		!cfg.DiscoveryEndpoint.IsNull() ||
		!cfg.ClientID.IsNull() ||
		!cfg.ClientSecret.IsNull()
}
func isSamlConfigSet(cfg *SamlConfigModel) bool {
	if cfg == nil {
		return false
	}
	return !cfg.Issuer.IsNull() ||
		!cfg.EntryPoint.IsNull() ||
		!cfg.CallbackURL.IsNull() ||
		!cfg.Cert.IsNull()
}

func validateIdentityProviderConfigChoice(oidc *OidcConfigModel, saml *SamlConfigModel) error {
	oidcSet := isOidcConfigSet(oidc)
	samlSet := isSamlConfigSet(saml)

	if !oidcSet && !samlSet {
		return fmt.Errorf("exactly one of oidc_config or saml_config must be set")
	}
	if oidcSet && samlSet {
		return fmt.Errorf("only one of oidc_config or saml_config can be set at a time")
	}
	return nil
}

func encodeAdditionalParams(m *map[string]interface{}) jsontypes.Normalized {
	if m == nil {
		return jsontypes.NewNormalizedNull()
	}
	b, err := json.Marshal(*m)
	if err != nil {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(b))
}

func stringValueOrNull(ptr *string) types.String {
	if ptr == nil {
		return types.StringNull()
	}
	return types.StringValue(*ptr)
}

func boolValueOrNull(ptr *bool) types.Bool {
	if ptr == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*ptr)
}

func mapStringToTypes(in *map[string]string) types.Map {
	if in == nil {
		return types.MapNull(types.StringType)
	}
	elems := make(map[string]attr.Value, len(*in))
	for k, v := range *in {
		elems[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, elems)
}

// AttrSpecs implements resourceWithAttrSpec — activates the schema↔AttrSpec
// drift lint for this resource.
func (r *IdentityProviderResource) AttrSpecs() []AttrSpec { return identityProviderAttrSpec }

// identityProviderAttrSpec declares the wire shape for `archestra_identity_provider`.
//
// Storage (per platform/backend/src/database/schemas/identity-provider.ts):
//   - oidc_config / saml_config / role_mapping / team_sync_config are
//     `text(...)` columns serialized as JSON via serializeConfigValue (per
//     identity-provider.ee.ts:697-749). Drizzle replaces the whole TEXT
//     column on update — there is no field-level merge inside. Modelled as
//     AtomicObject; merge-patch emits the whole nested object on any change.
//   - All other fields are top-level columns → Scalar.
//
// Sensitive fields inside AtomicObjects (per A7 + A9): masked on emit via
// LogPatch. State is set from plan (for Create) or response (for Read);
// plan-based state for sensitive fields is correct because the plan IS the
// user-supplied value.
var identityProviderAttrSpec = []AttrSpec{
	{TFName: "provider_id", JSONName: "providerId", Kind: Scalar},
	{TFName: "domain", JSONName: "domain", Kind: Scalar},
	{TFName: "domain_verified", JSONName: "domainVerified", Kind: Scalar},
	{TFName: "issuer", JSONName: "issuer", Kind: Scalar},
	{TFName: "user_id", JSONName: "userId", Kind: Scalar},
	{TFName: "organization_id", JSONName: "organizationId", Kind: Scalar},
	{
		TFName: "oidc_config", JSONName: "oidcConfig", Kind: AtomicObject, OmitOnNull: true,
		Children: []AttrSpec{
			{TFName: "issuer", JSONName: "issuer", Kind: Scalar},
			{TFName: "discovery_endpoint", JSONName: "discoveryEndpoint", Kind: Scalar},
			{TFName: "client_id", JSONName: "clientId", Kind: Scalar},
			{TFName: "client_secret", JSONName: "clientSecret", Kind: Scalar, Sensitive: true},
			{TFName: "authorization_endpoint", JSONName: "authorizationEndpoint", Kind: Scalar},
			{TFName: "token_endpoint", JSONName: "tokenEndpoint", Kind: Scalar},
			{TFName: "user_info_endpoint", JSONName: "userInfoEndpoint", Kind: Scalar},
			{TFName: "jwks_endpoint", JSONName: "jwksEndpoint", Kind: Scalar},
			{TFName: "scopes", JSONName: "scopes", Kind: List},
			{TFName: "pkce", JSONName: "pkce", Kind: Scalar},
			{TFName: "override_user_info", JSONName: "overrideUserInfo", Kind: Scalar},
			{TFName: "skip_discovery", JSONName: "skipDiscovery", Kind: Scalar},
			{TFName: "enable_rp_initiated_logout", JSONName: "enableRpInitiatedLogout", Kind: Scalar},
			{TFName: "hd", JSONName: "hd", Kind: Scalar},
			{TFName: "token_endpoint_authentication", JSONName: "tokenEndpointAuthentication", Kind: Scalar},
			{TFName: "mapping", JSONName: "mapping", Kind: AtomicObject, Children: oidcMappingChildren},
			{TFName: "enterprise_managed_credentials", JSONName: "enterpriseManagedCredentials", Kind: AtomicObject, Children: emcChildren},
		},
	},
	{
		TFName: "saml_config", JSONName: "samlConfig", Kind: AtomicObject, OmitOnNull: true,
		// Backend zod (shared/identity-provider.ts:150) requires `spMetadata`
		// as a non-optional object — sub-fields may all be omitted, but the
		// object itself must be present. Inject `{}` if the user didn't set
		// sp_metadata so we don't 400 with "received undefined".
		Encoder: ensureSamlSpMetadata,
		Children: []AttrSpec{
			{TFName: "issuer", JSONName: "issuer", Kind: Scalar},
			{TFName: "entry_point", JSONName: "entryPoint", Kind: Scalar},
			{TFName: "callback_url", JSONName: "callbackUrl", Kind: Scalar},
			{TFName: "cert", JSONName: "cert", Kind: Scalar, Sensitive: true},
			{TFName: "audience", JSONName: "audience", Kind: Scalar},
			{TFName: "digest_algorithm", JSONName: "digestAlgorithm", Kind: Scalar},
			{TFName: "identifier_format", JSONName: "identifierFormat", Kind: Scalar},
			{TFName: "decryption_pvk", JSONName: "decryptionPvk", Kind: Scalar, Sensitive: true},
			{TFName: "private_key", JSONName: "privateKey", Kind: Scalar, Sensitive: true},
			{TFName: "signature_algorithm", JSONName: "signatureAlgorithm", Kind: Scalar},
			{TFName: "want_assertions_signed", JSONName: "wantAssertionsSigned", Kind: Scalar},
			{TFName: "idp_metadata", JSONName: "idpMetadata", Kind: AtomicObject, Children: samlIdpMetadataChildren},
			{TFName: "sp_metadata", JSONName: "spMetadata", Kind: AtomicObject, Children: samlSpMetadataChildren},
			{TFName: "mapping", JSONName: "mapping", Kind: AtomicObject, Children: samlMappingChildren},
			{
				TFName: "additional_params", JSONName: "additionalParams", Kind: Scalar,
				// `additional_params` is a JSON-stringified object on the TF side
				// (jsontypes.Normalized). Decode it before emission so the wire form
				// is a real JSON object rather than a string-encoded JSON object.
				Encoder: encodeAdditionalParamsValue,
			},
		},
	},
	{TFName: "role_mapping", JSONName: "roleMapping", Kind: AtomicObject, OmitOnNull: true, Children: roleMappingChildren},
	{TFName: "team_sync_config", JSONName: "teamSyncConfig", Kind: AtomicObject, OmitOnNull: true, Children: teamSyncConfigChildren},
}

var oidcMappingChildren = []AttrSpec{
	{TFName: "email", JSONName: "email", Kind: Scalar},
	{TFName: "email_verified", JSONName: "emailVerified", Kind: Scalar},
	{TFName: "extra_fields", JSONName: "extraFields", Kind: Map},
	{TFName: "id", JSONName: "id", Kind: Scalar},
	{TFName: "image", JSONName: "image", Kind: Scalar},
	{TFName: "name", JSONName: "name", Kind: Scalar},
}

var emcChildren = []AttrSpec{
	{TFName: "exchange_strategy", JSONName: "exchangeStrategy", Kind: Scalar},
	{TFName: "client_id", JSONName: "clientId", Kind: Scalar},
	{TFName: "client_secret", JSONName: "clientSecret", Kind: Scalar, Sensitive: true},
	{TFName: "token_endpoint", JSONName: "tokenEndpoint", Kind: Scalar},
	{TFName: "token_endpoint_authentication", JSONName: "tokenEndpointAuthentication", Kind: Scalar},
	{TFName: "private_key_pem", JSONName: "privateKeyPem", Kind: Scalar, Sensitive: true},
	{TFName: "private_key_id", JSONName: "privateKeyId", Kind: Scalar},
	{TFName: "client_assertion_audience", JSONName: "clientAssertionAudience", Kind: Scalar},
	{TFName: "subject_token_type", JSONName: "subjectTokenType", Kind: Scalar},
}

var samlIdpMetadataChildren = []AttrSpec{
	{TFName: "cert", JSONName: "cert", Kind: Scalar, Sensitive: true},
	{TFName: "enc_private_key", JSONName: "encPrivateKey", Kind: Scalar, Sensitive: true},
	{TFName: "enc_private_key_pass", JSONName: "encPrivateKeyPass", Kind: Scalar, Sensitive: true},
	{TFName: "entity_id", JSONName: "entityID", Kind: Scalar},
	{TFName: "entity_url", JSONName: "entityURL", Kind: Scalar},
	{TFName: "is_assertion_encrypted", JSONName: "isAssertionEncrypted", Kind: Scalar},
	{TFName: "metadata", JSONName: "metadata", Kind: Scalar},
	{TFName: "private_key", JSONName: "privateKey", Kind: Scalar, Sensitive: true},
	{TFName: "private_key_pass", JSONName: "privateKeyPass", Kind: Scalar, Sensitive: true},
	{TFName: "redirect_url", JSONName: "redirectURL", Kind: Scalar},
	{
		TFName: "single_sign_on_service", JSONName: "singleSignOnService", Kind: List,
		Children: []AttrSpec{
			{TFName: "binding", JSONName: "Binding", Kind: Scalar},
			{TFName: "location", JSONName: "Location", Kind: Scalar},
		},
	},
}

var samlSpMetadataChildren = []AttrSpec{
	{TFName: "binding", JSONName: "binding", Kind: Scalar},
	{TFName: "enc_private_key", JSONName: "encPrivateKey", Kind: Scalar, Sensitive: true},
	{TFName: "enc_private_key_pass", JSONName: "encPrivateKeyPass", Kind: Scalar, Sensitive: true},
	{TFName: "entity_id", JSONName: "entityID", Kind: Scalar},
	{TFName: "is_assertion_encrypted", JSONName: "isAssertionEncrypted", Kind: Scalar},
	{TFName: "metadata", JSONName: "metadata", Kind: Scalar},
	{TFName: "private_key", JSONName: "privateKey", Kind: Scalar, Sensitive: true},
	{TFName: "private_key_pass", JSONName: "privateKeyPass", Kind: Scalar, Sensitive: true},
}

var samlMappingChildren = []AttrSpec{
	{TFName: "email", JSONName: "email", Kind: Scalar},
	{TFName: "email_verified", JSONName: "emailVerified", Kind: Scalar},
	{TFName: "extra_fields", JSONName: "extraFields", Kind: Map},
	{TFName: "first_name", JSONName: "firstName", Kind: Scalar},
	{TFName: "id", JSONName: "id", Kind: Scalar},
	{TFName: "last_name", JSONName: "lastName", Kind: Scalar},
	{TFName: "name", JSONName: "name", Kind: Scalar},
}

var roleMappingChildren = []AttrSpec{
	{TFName: "default_role", JSONName: "defaultRole", Kind: Scalar},
	{TFName: "skip_role_sync", JSONName: "skipRoleSync", Kind: Scalar},
	{TFName: "strict_mode", JSONName: "strictMode", Kind: Scalar},
	{
		TFName: "rules", JSONName: "rules", Kind: List,
		Children: []AttrSpec{
			{TFName: "expression", JSONName: "expression", Kind: Scalar},
			{TFName: "role", JSONName: "role", Kind: Scalar},
		},
	},
}

var teamSyncConfigChildren = []AttrSpec{
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
	{TFName: "groups_expression", JSONName: "groupsExpression", Kind: Scalar},
}

// encodeAdditionalParamsValue parses the JSON-string `additional_params`
// value into a real JSON object before merge-patch emits it. The TF type is
// `jsontypes.Normalized` (string), but the wire form is a JSON object.
func encodeAdditionalParamsValue(v any) any {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return s
	}
	return parsed
}

// ensureSamlSpMetadata guarantees the merge-patch payload's
// `samlConfig.spMetadata` is at least an empty object. The backend zod
// schema marks spMetadata as required (sub-fields are individually
// optional, but the object itself must be present). When the user omits
// the sp_metadata block, our default child-walk skips the null sub-field
// and the wire form lacks spMetadata entirely, which the backend rejects
// with `Invalid input: expected object, received undefined`.
func ensureSamlSpMetadata(v any) any {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return v
	}
	if _, has := m["spMetadata"]; !has {
		m["spMetadata"] = map[string]any{}
	}
	return m
}
