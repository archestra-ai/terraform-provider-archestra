package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SsoProviderResource{}
var _ resource.ResourceWithImportState = &SsoProviderResource{}

func NewSsoProviderResource() resource.Resource {
	return &SsoProviderResource{}
}

// SsoProviderResource manages SSO providers (OIDC or SAML).
type SsoProviderResource struct {
	client *client.ClientWithResponses
}

type SsoProviderResourceModel struct {
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
	Issuer                      types.String      `tfsdk:"issuer"`
	DiscoveryEndpoint           types.String      `tfsdk:"discovery_endpoint"`
	ClientID                    types.String      `tfsdk:"client_id"`
	ClientSecret                types.String      `tfsdk:"client_secret"`
	AuthorizationEndpoint       types.String      `tfsdk:"authorization_endpoint"`
	TokenEndpoint               types.String      `tfsdk:"token_endpoint"`
	UserInfoEndpoint            types.String      `tfsdk:"user_info_endpoint"`
	JwksEndpoint                types.String      `tfsdk:"jwks_endpoint"`
	Scopes                      []types.String    `tfsdk:"scopes"`
	Pkce                        types.Bool        `tfsdk:"pkce"`
	OverrideUserInfo            types.Bool        `tfsdk:"override_user_info"`
	TokenEndpointAuthentication types.String      `tfsdk:"token_endpoint_authentication"`
	Mapping                     *OidcMappingModel `tfsdk:"mapping"`
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
	Issuer               types.String      `tfsdk:"issuer"`
	EntryPoint           types.String      `tfsdk:"entry_point"`
	CallbackURL          types.String      `tfsdk:"callback_url"`
	Cert                 types.String      `tfsdk:"cert"`
	Audience             types.String      `tfsdk:"audience"`
	DigestAlgorithm      types.String      `tfsdk:"digest_algorithm"`
	IdentifierFormat     types.String      `tfsdk:"identifier_format"`
	DecryptionPvk        types.String      `tfsdk:"decryption_pvk"`
	PrivateKey           types.String      `tfsdk:"private_key"`
	SignatureAlgorithm   types.String      `tfsdk:"signature_algorithm"`
	WantAssertionsSigned types.Bool        `tfsdk:"want_assertions_signed"`
	IdpMetadata          *SamlIdpMetadata  `tfsdk:"idp_metadata"`
	SpMetadata           *SamlSpMetadata   `tfsdk:"sp_metadata"`
	Mapping              *SamlMappingModel `tfsdk:"mapping"`
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

func (r *SsoProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_provider"
}

func (r *SsoProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages Archestra SSO providers (OIDC or SAML). Exactly one of oidc_config or saml_config must be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SSO provider identifier",
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
					"token_endpoint_authentication": schema.StringAttribute{Optional: true, MarkdownDescription: "Token endpoint auth method (client_secret_basic or client_secret_post)."},
				},
				Blocks: map[string]schema.Block{
					"mapping": schema.SingleNestedBlock{
						MarkdownDescription: "Attribute mapping for user fields.",
						Attributes: map[string]schema.Attribute{
							"email":          schema.StringAttribute{Optional: true},
							"email_verified": schema.StringAttribute{Optional: true},
							"extra_fields":   schema.MapAttribute{Optional: true, ElementType: types.StringType},
							"id":             schema.StringAttribute{Optional: true},
							"image":          schema.StringAttribute{Optional: true},
							"name":           schema.StringAttribute{Optional: true},
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
						MarkdownDescription: "Attribute mapping for user fields.",
						Attributes: map[string]schema.Attribute{
							"email":          schema.StringAttribute{Optional: true},
							"email_verified": schema.StringAttribute{Optional: true},
							"extra_fields":   schema.MapAttribute{Optional: true, ElementType: types.StringType},
							"first_name":     schema.StringAttribute{Optional: true},
							"id":             schema.StringAttribute{Optional: true},
							"last_name":      schema.StringAttribute{Optional: true},
							"name":           schema.StringAttribute{Optional: true},
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

func (r *SsoProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SsoProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SsoProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateSsoConfigChoice(data.OidcConfig, data.SamlConfig); err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}

	body := client.CreateSsoProviderJSONRequestBody{
		Domain:     data.Domain.ValueString(),
		Issuer:     data.Issuer.ValueString(),
		ProviderId: data.ProviderID.ValueString(),
	}

	if data.OidcConfig != nil {
		body.OidcConfig = expandOidcConfigCreate(*data.OidcConfig)
	}

	if data.SamlConfig != nil {
		body.SamlConfig = expandSamlConfigCreate(*data.SamlConfig)
	}

	if data.RoleMapping != nil {
		body.RoleMapping = expandRoleMapping(data.RoleMapping)
	}

	if data.TeamSyncConfig != nil {
		body.TeamSyncConfig = expandTeamSyncConfig(data.TeamSyncConfig)
	}

	apiResp, err := r.client.CreateSsoProviderWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create SSO provider: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
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

func (r *SsoProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SsoProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetSsoProviderWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read SSO provider: %s", err))
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

	newState := state
	newState.ID = types.StringValue(apiResp.JSON200.Id)
	newState.ProviderID = types.StringValue(apiResp.JSON200.ProviderId)
	newState.Domain = types.StringValue(apiResp.JSON200.Domain)
	newState.Issuer = types.StringValue(apiResp.JSON200.Issuer)

	if apiResp.JSON200.DomainVerified != nil {
		newState.DomainVerified = types.BoolValue(*apiResp.JSON200.DomainVerified)
	} else {
		newState.DomainVerified = state.DomainVerified
	}
	if apiResp.JSON200.OrganizationId != nil {
		newState.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	}
	if apiResp.JSON200.UserId != nil {
		newState.UserID = types.StringValue(*apiResp.JSON200.UserId)
	}

	// Map OIDC config if present
	if apiResp.JSON200.OidcConfig != nil {
		oidcCfg := apiResp.JSON200.OidcConfig
		newState.OidcConfig = &OidcConfigModel{
			Issuer:            types.StringValue(oidcCfg.Issuer),
			DiscoveryEndpoint: types.StringValue(oidcCfg.DiscoveryEndpoint),
			ClientID:          types.StringValue(oidcCfg.ClientId),
			ClientSecret: preserveSensitive(oidcCfg.ClientSecret, &state, func(p *SsoProviderResourceModel) types.String {
				if p != nil && p.OidcConfig != nil {
					return p.OidcConfig.ClientSecret
				}
				return types.StringNull()
			}),
			Pkce: types.BoolValue(oidcCfg.Pkce),
		}

		// Preserve user input for optional fields if they were set in the prior state
		if state.OidcConfig != nil {
			// For override_user_info, prefer the prior state since it was explicitly configured
			if !state.OidcConfig.OverrideUserInfo.IsNull() {
				newState.OidcConfig.OverrideUserInfo = state.OidcConfig.OverrideUserInfo
			} else if oidcCfg.OverrideUserInfo != nil {
				newState.OidcConfig.OverrideUserInfo = types.BoolValue(*oidcCfg.OverrideUserInfo)
			}
			// Preserve other optional fields from prior state
			if !state.OidcConfig.AuthorizationEndpoint.IsNull() {
				newState.OidcConfig.AuthorizationEndpoint = state.OidcConfig.AuthorizationEndpoint
			} else if oidcCfg.AuthorizationEndpoint != nil {
				newState.OidcConfig.AuthorizationEndpoint = types.StringValue(*oidcCfg.AuthorizationEndpoint)
			}
			if !state.OidcConfig.TokenEndpoint.IsNull() {
				newState.OidcConfig.TokenEndpoint = state.OidcConfig.TokenEndpoint
			} else if oidcCfg.TokenEndpoint != nil {
				newState.OidcConfig.TokenEndpoint = types.StringValue(*oidcCfg.TokenEndpoint)
			}
			if !state.OidcConfig.UserInfoEndpoint.IsNull() {
				newState.OidcConfig.UserInfoEndpoint = state.OidcConfig.UserInfoEndpoint
			} else if oidcCfg.UserInfoEndpoint != nil {
				newState.OidcConfig.UserInfoEndpoint = types.StringValue(*oidcCfg.UserInfoEndpoint)
			}
			if !state.OidcConfig.JwksEndpoint.IsNull() {
				newState.OidcConfig.JwksEndpoint = state.OidcConfig.JwksEndpoint
			} else if oidcCfg.JwksEndpoint != nil {
				newState.OidcConfig.JwksEndpoint = types.StringValue(*oidcCfg.JwksEndpoint)
			}
			if len(state.OidcConfig.Scopes) > 0 {
				newState.OidcConfig.Scopes = state.OidcConfig.Scopes
			} else if oidcCfg.Scopes != nil {
				scopes := make([]types.String, len(*oidcCfg.Scopes))
				for i, scope := range *oidcCfg.Scopes {
					scopes[i] = types.StringValue(scope)
				}
				newState.OidcConfig.Scopes = scopes
			}
			if !state.OidcConfig.TokenEndpointAuthentication.IsNull() {
				newState.OidcConfig.TokenEndpointAuthentication = state.OidcConfig.TokenEndpointAuthentication
			} else if oidcCfg.TokenEndpointAuthentication != nil {
				newState.OidcConfig.TokenEndpointAuthentication = types.StringValue(string(*oidcCfg.TokenEndpointAuthentication))
			}
			// Preserve mapping from prior state - user may have configured it
			if state.OidcConfig.Mapping != nil {
				newState.OidcConfig.Mapping = state.OidcConfig.Mapping
			} else if oidcCfg.Mapping != nil {
				mapping := oidcCfg.Mapping
				newState.OidcConfig.Mapping = &OidcMappingModel{
					Email:         stringValueOrNull(mapping.Email),
					EmailVerified: stringValueOrNull(mapping.EmailVerified),
					ExtraFields:   mapStringToTypes(mapping.ExtraFields),
					ID:            stringValueOrNull(mapping.Id),
					Image:         stringValueOrNull(mapping.Image),
					Name:          stringValueOrNull(mapping.Name),
				}
			}
		} else {
			// If there was no prior OIDC config, populate from API response
			if oidcCfg.AuthorizationEndpoint != nil {
				newState.OidcConfig.AuthorizationEndpoint = types.StringValue(*oidcCfg.AuthorizationEndpoint)
			}
			if oidcCfg.TokenEndpoint != nil {
				newState.OidcConfig.TokenEndpoint = types.StringValue(*oidcCfg.TokenEndpoint)
			}
			if oidcCfg.UserInfoEndpoint != nil {
				newState.OidcConfig.UserInfoEndpoint = types.StringValue(*oidcCfg.UserInfoEndpoint)
			}
			if oidcCfg.JwksEndpoint != nil {
				newState.OidcConfig.JwksEndpoint = types.StringValue(*oidcCfg.JwksEndpoint)
			}
			if oidcCfg.Scopes != nil {
				scopes := make([]types.String, len(*oidcCfg.Scopes))
				for i, scope := range *oidcCfg.Scopes {
					scopes[i] = types.StringValue(scope)
				}
				newState.OidcConfig.Scopes = scopes
			}
			if oidcCfg.OverrideUserInfo != nil {
				newState.OidcConfig.OverrideUserInfo = types.BoolValue(*oidcCfg.OverrideUserInfo)
			}
			if oidcCfg.TokenEndpointAuthentication != nil {
				newState.OidcConfig.TokenEndpointAuthentication = types.StringValue(string(*oidcCfg.TokenEndpointAuthentication))
			}
			if oidcCfg.Mapping != nil {
				mapping := oidcCfg.Mapping
				newState.OidcConfig.Mapping = &OidcMappingModel{
					Email:         stringValueOrNull(mapping.Email),
					EmailVerified: stringValueOrNull(mapping.EmailVerified),
					ExtraFields:   mapStringToTypes(mapping.ExtraFields),
					ID:            stringValueOrNull(mapping.Id),
					Image:         stringValueOrNull(mapping.Image),
					Name:          stringValueOrNull(mapping.Name),
				}
			}
		}
	}

	// Map SAML config if present
	if apiResp.JSON200.SamlConfig != nil {
		samlCfg := apiResp.JSON200.SamlConfig
		newState.SamlConfig = &SamlConfigModel{
			Issuer:               types.StringValue(samlCfg.Issuer),
			EntryPoint:           types.StringValue(samlCfg.EntryPoint),
			CallbackURL:          types.StringValue(samlCfg.CallbackUrl),
			Cert:                 types.StringValue(samlCfg.Cert),
			Audience:             stringValueOrNull(samlCfg.Audience),
			DigestAlgorithm:      stringValueOrNull(samlCfg.DigestAlgorithm),
			IdentifierFormat:     stringValueOrNull(samlCfg.IdentifierFormat),
			DecryptionPvk:        stringValueOrNull(samlCfg.DecryptionPvk),
			PrivateKey:           stringValueOrNull(samlCfg.PrivateKey),
			SignatureAlgorithm:   stringValueOrNull(samlCfg.SignatureAlgorithm),
			WantAssertionsSigned: boolValueOrNull(samlCfg.WantAssertionsSigned),
		}

		// Map IdpMetadata if present
		if samlCfg.IdpMetadata != nil {
			idpMeta := samlCfg.IdpMetadata
			newState.SamlConfig.IdpMetadata = &SamlIdpMetadata{
				EntityID:             stringValueOrNull(idpMeta.EntityID),
				EncPrivateKey:        stringValueOrNull(idpMeta.EncPrivateKey),
				EncPrivateKeyPass:    stringValueOrNull(idpMeta.EncPrivateKeyPass),
				EntityURL:            stringValueOrNull(idpMeta.EntityURL),
				IsAssertionEncrypted: boolValueOrNull(idpMeta.IsAssertionEncrypted),
				Metadata:             stringValueOrNull(idpMeta.Metadata),
				PrivateKey:           stringValueOrNull(idpMeta.PrivateKey),
				PrivateKeyPass:       stringValueOrNull(idpMeta.PrivateKeyPass),
				RedirectURL:          stringValueOrNull(idpMeta.RedirectURL),
				Cert:                 stringValueOrNull(idpMeta.Cert),
			}
			if idpMeta.SingleSignOnService != nil {
				services := make([]SsoService, len(*idpMeta.SingleSignOnService))
				for i, svc := range *idpMeta.SingleSignOnService {
					services[i] = SsoService{
						Binding:  types.StringValue(svc.Binding),
						Location: types.StringValue(svc.Location),
					}
				}
				newState.SamlConfig.IdpMetadata.SingleSignOnService = services
			}
		}

		// Map SpMetadata if present
		if samlCfg.SpMetadata.Metadata != nil || samlCfg.SpMetadata.EntityID != nil {
			spMeta := samlCfg.SpMetadata
			newState.SamlConfig.SpMetadata = &SamlSpMetadata{
				Binding:              stringValueOrNull(spMeta.Binding),
				EncPrivateKey:        stringValueOrNull(spMeta.EncPrivateKey),
				EncPrivateKeyPass:    stringValueOrNull(spMeta.EncPrivateKeyPass),
				EntityID:             stringValueOrNull(spMeta.EntityID),
				IsAssertionEncrypted: boolValueOrNull(spMeta.IsAssertionEncrypted),
				Metadata:             stringValueOrNull(spMeta.Metadata),
				PrivateKey:           stringValueOrNull(spMeta.PrivateKey),
				PrivateKeyPass:       stringValueOrNull(spMeta.PrivateKeyPass),
			}
		}

		// Map Mapping if present - preserve from prior state to avoid spurious diffs
		if state.SamlConfig != nil && state.SamlConfig.Mapping != nil {
			newState.SamlConfig.Mapping = state.SamlConfig.Mapping
		} else if samlCfg.Mapping != nil {
			mapping := samlCfg.Mapping
			newState.SamlConfig.Mapping = &SamlMappingModel{
				Email:         stringValueOrNull(mapping.Email),
				EmailVerified: stringValueOrNull(mapping.EmailVerified),
				ExtraFields:   mapStringToTypes(mapping.ExtraFields),
				FirstName:     stringValueOrNull(mapping.FirstName),
				ID:            stringValueOrNull(mapping.Id),
				LastName:      stringValueOrNull(mapping.LastName),
				Name:          stringValueOrNull(mapping.Name),
			}
		}
	}

	// Map role mapping if present
	if apiResp.JSON200.RoleMapping != nil {
		rm := apiResp.JSON200.RoleMapping
		newState.RoleMapping = &RoleMappingModel{
			DefaultRole:  stringValueOrNull(rm.DefaultRole),
			SkipRoleSync: boolValueOrNull(rm.SkipRoleSync),
			StrictMode:   boolValueOrNull(rm.StrictMode),
		}
		if rm.Rules != nil {
			rules := make([]RoleRuleModel, len(*rm.Rules))
			for i, rule := range *rm.Rules {
				rules[i] = RoleRuleModel{
					Expression: types.StringValue(rule.Expression),
					Role:       types.StringValue(rule.Role),
				}
			}
			newState.RoleMapping.Rules = rules
		}
	} else if state.RoleMapping != nil {
		// Preserve role mapping from prior state if API doesn't return it
		newState.RoleMapping = state.RoleMapping
	}

	// Map team sync config if present
	if apiResp.JSON200.TeamSyncConfig != nil {
		tsc := apiResp.JSON200.TeamSyncConfig
		newState.TeamSyncConfig = &TeamSyncConfigModel{
			Enabled:          boolValueOrNull(tsc.Enabled),
			GroupsExpression: stringValueOrNull(tsc.GroupsExpression),
		}
	} else if state.TeamSyncConfig != nil {
		// Preserve team sync config from prior state if API doesn't return it
		newState.TeamSyncConfig = state.TeamSyncConfig
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *SsoProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SsoProviderResourceModel
	var state SsoProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateSsoConfigChoice(plan.OidcConfig, plan.SamlConfig); err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}

	body := client.UpdateSsoProviderJSONRequestBody{}

	if !plan.Domain.IsNull() {
		domain := plan.Domain.ValueString()
		body.Domain = &domain
	}

	if !plan.Issuer.IsNull() {
		issuer := plan.Issuer.ValueString()
		body.Issuer = &issuer
	}

	if !plan.ProviderID.IsNull() {
		pid := plan.ProviderID.ValueString()
		body.ProviderId = &pid
	}

	if plan.OidcConfig != nil {
		body.OidcConfig = expandOidcConfigUpdate(*plan.OidcConfig)
	}

	if plan.SamlConfig != nil {
		body.SamlConfig = expandSamlConfigUpdate(*plan.SamlConfig)
	}

	if plan.RoleMapping != nil {
		body.RoleMapping = expandRoleMapping(plan.RoleMapping)
	}

	if plan.TeamSyncConfig != nil {
		body.TeamSyncConfig = expandTeamSyncConfig(plan.TeamSyncConfig)
	}

	apiResp, err := r.client.UpdateSsoProviderWithResponse(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update SSO provider: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	newState := plan
	newState.ID = types.StringValue(apiResp.JSON200.Id)
	newState.ProviderID = types.StringValue(apiResp.JSON200.ProviderId)
	newState.Domain = types.StringValue(apiResp.JSON200.Domain)
	newState.Issuer = types.StringValue(apiResp.JSON200.Issuer)

	if apiResp.JSON200.DomainVerified != nil {
		newState.DomainVerified = types.BoolValue(*apiResp.JSON200.DomainVerified)
	}
	if apiResp.JSON200.OrganizationId != nil {
		newState.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	}
	if apiResp.JSON200.UserId != nil {
		newState.UserID = types.StringValue(*apiResp.JSON200.UserId)
	}

	// Map OIDC config if present (similar to Read method)
	if apiResp.JSON200.OidcConfig != nil {
		oidcCfg := apiResp.JSON200.OidcConfig
		newState.OidcConfig = &OidcConfigModel{
			Issuer:            types.StringValue(oidcCfg.Issuer),
			DiscoveryEndpoint: types.StringValue(oidcCfg.DiscoveryEndpoint),
			ClientID:          types.StringValue(oidcCfg.ClientId),
			ClientSecret: preserveSensitive(oidcCfg.ClientSecret, &state, func(p *SsoProviderResourceModel) types.String {
				if p != nil && p.OidcConfig != nil {
					return p.OidcConfig.ClientSecret
				}
				return types.StringNull()
			}),
			Pkce:             types.BoolValue(oidcCfg.Pkce),
			OverrideUserInfo: boolValueOrNull(oidcCfg.OverrideUserInfo),
		}

		if oidcCfg.AuthorizationEndpoint != nil {
			newState.OidcConfig.AuthorizationEndpoint = types.StringValue(*oidcCfg.AuthorizationEndpoint)
		}
		if oidcCfg.TokenEndpoint != nil {
			newState.OidcConfig.TokenEndpoint = types.StringValue(*oidcCfg.TokenEndpoint)
		}
		if oidcCfg.UserInfoEndpoint != nil {
			newState.OidcConfig.UserInfoEndpoint = types.StringValue(*oidcCfg.UserInfoEndpoint)
		}
		if oidcCfg.JwksEndpoint != nil {
			newState.OidcConfig.JwksEndpoint = types.StringValue(*oidcCfg.JwksEndpoint)
		}
		if oidcCfg.Scopes != nil {
			scopes := make([]types.String, len(*oidcCfg.Scopes))
			for i, scope := range *oidcCfg.Scopes {
				scopes[i] = types.StringValue(scope)
			}
			newState.OidcConfig.Scopes = scopes
		}
		if oidcCfg.TokenEndpointAuthentication != nil {
			newState.OidcConfig.TokenEndpointAuthentication = types.StringValue(string(*oidcCfg.TokenEndpointAuthentication))
		}
	}

	// Map SAML config if present (similar to Read method)
	if apiResp.JSON200.SamlConfig != nil {
		samlCfg := apiResp.JSON200.SamlConfig
		newState.SamlConfig = &SamlConfigModel{
			Issuer:               types.StringValue(samlCfg.Issuer),
			EntryPoint:           types.StringValue(samlCfg.EntryPoint),
			CallbackURL:          types.StringValue(samlCfg.CallbackUrl),
			Cert:                 types.StringValue(samlCfg.Cert),
			Audience:             stringValueOrNull(samlCfg.Audience),
			DigestAlgorithm:      stringValueOrNull(samlCfg.DigestAlgorithm),
			IdentifierFormat:     stringValueOrNull(samlCfg.IdentifierFormat),
			DecryptionPvk:        stringValueOrNull(samlCfg.DecryptionPvk),
			PrivateKey:           stringValueOrNull(samlCfg.PrivateKey),
			SignatureAlgorithm:   stringValueOrNull(samlCfg.SignatureAlgorithm),
			WantAssertionsSigned: boolValueOrNull(samlCfg.WantAssertionsSigned),
		}

		// Map IdpMetadata if present
		if samlCfg.IdpMetadata != nil {
			idpMeta := samlCfg.IdpMetadata
			newState.SamlConfig.IdpMetadata = &SamlIdpMetadata{
				EntityID:             stringValueOrNull(idpMeta.EntityID),
				EncPrivateKey:        stringValueOrNull(idpMeta.EncPrivateKey),
				EncPrivateKeyPass:    stringValueOrNull(idpMeta.EncPrivateKeyPass),
				EntityURL:            stringValueOrNull(idpMeta.EntityURL),
				IsAssertionEncrypted: boolValueOrNull(idpMeta.IsAssertionEncrypted),
				Metadata:             stringValueOrNull(idpMeta.Metadata),
				PrivateKey:           stringValueOrNull(idpMeta.PrivateKey),
				PrivateKeyPass:       stringValueOrNull(idpMeta.PrivateKeyPass),
				RedirectURL:          stringValueOrNull(idpMeta.RedirectURL),
				Cert:                 stringValueOrNull(idpMeta.Cert),
			}
			if idpMeta.SingleSignOnService != nil {
				services := make([]SsoService, len(*idpMeta.SingleSignOnService))
				for i, svc := range *idpMeta.SingleSignOnService {
					services[i] = SsoService{
						Binding:  types.StringValue(svc.Binding),
						Location: types.StringValue(svc.Location),
					}
				}
				newState.SamlConfig.IdpMetadata.SingleSignOnService = services
			}
		}

		// Map SpMetadata if present
		if samlCfg.SpMetadata.Metadata != nil || samlCfg.SpMetadata.EntityID != nil {
			spMeta := samlCfg.SpMetadata
			newState.SamlConfig.SpMetadata = &SamlSpMetadata{
				Binding:              stringValueOrNull(spMeta.Binding),
				EncPrivateKey:        stringValueOrNull(spMeta.EncPrivateKey),
				EncPrivateKeyPass:    stringValueOrNull(spMeta.EncPrivateKeyPass),
				EntityID:             stringValueOrNull(spMeta.EntityID),
				IsAssertionEncrypted: boolValueOrNull(spMeta.IsAssertionEncrypted),
				Metadata:             stringValueOrNull(spMeta.Metadata),
				PrivateKey:           stringValueOrNull(spMeta.PrivateKey),
				PrivateKeyPass:       stringValueOrNull(spMeta.PrivateKeyPass),
			}
		}

		// Map Mapping if present - preserve from prior state to avoid spurious diffs
		if plan.SamlConfig != nil && plan.SamlConfig.Mapping != nil {
			newState.SamlConfig.Mapping = plan.SamlConfig.Mapping
		} else if samlCfg.Mapping != nil {
			mapping := samlCfg.Mapping
			newState.SamlConfig.Mapping = &SamlMappingModel{
				Email:         stringValueOrNull(mapping.Email),
				EmailVerified: stringValueOrNull(mapping.EmailVerified),
				ExtraFields:   mapStringToTypes(mapping.ExtraFields),
				FirstName:     stringValueOrNull(mapping.FirstName),
				ID:            stringValueOrNull(mapping.Id),
				LastName:      stringValueOrNull(mapping.LastName),
				Name:          stringValueOrNull(mapping.Name),
			}
		}
	}

	// Map role mapping if present (similar to Read method)
	if apiResp.JSON200.RoleMapping != nil {
		rm := apiResp.JSON200.RoleMapping
		newState.RoleMapping = &RoleMappingModel{
			DefaultRole:  stringValueOrNull(rm.DefaultRole),
			SkipRoleSync: boolValueOrNull(rm.SkipRoleSync),
			StrictMode:   boolValueOrNull(rm.StrictMode),
		}
		if rm.Rules != nil {
			rules := make([]RoleRuleModel, len(*rm.Rules))
			for i, rule := range *rm.Rules {
				rules[i] = RoleRuleModel{
					Expression: types.StringValue(rule.Expression),
					Role:       types.StringValue(rule.Role),
				}
			}
			newState.RoleMapping.Rules = rules
		}
	}

	// Map team sync config if present (similar to Read method)
	if apiResp.JSON200.TeamSyncConfig != nil {
		tsc := apiResp.JSON200.TeamSyncConfig
		newState.TeamSyncConfig = &TeamSyncConfigModel{
			Enabled:          boolValueOrNull(tsc.Enabled),
			GroupsExpression: stringValueOrNull(tsc.GroupsExpression),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *SsoProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SsoProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteSsoProviderWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete SSO provider: %s", err))
		return
	}

	if apiResp.StatusCode() != 200 && apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Delete returned status %d", apiResp.StatusCode()))
	}
}

func (r *SsoProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helpers

func validateSsoConfigChoice(oidc *OidcConfigModel, saml *SamlConfigModel) error {
	count := 0
	if oidc != nil {
		count++
	}
	if saml != nil {
		count++
	}
	if count == 0 {
		return fmt.Errorf("exactly one of oidc_config or saml_config must be set")
	}
	if count > 1 {
		return fmt.Errorf("only one of oidc_config or saml_config can be set at a time")
	}
	return nil
}

func expandOidcConfigCreate(cfg OidcConfigModel) *struct {
	AuthorizationEndpoint *string `json:"authorizationEndpoint,omitempty"`
	ClientId              string  `json:"clientId"`
	ClientSecret          string  `json:"clientSecret"`
	DiscoveryEndpoint     string  `json:"discoveryEndpoint"`
	Issuer                string  `json:"issuer"`
	JwksEndpoint          *string `json:"jwksEndpoint,omitempty"`
	Mapping               *struct {
		Email         *string            `json:"email,omitempty"`
		EmailVerified *string            `json:"emailVerified,omitempty"`
		ExtraFields   *map[string]string `json:"extraFields,omitempty"`
		Id            *string            `json:"id,omitempty"`
		Image         *string            `json:"image,omitempty"`
		Name          *string            `json:"name,omitempty"`
	} `json:"mapping,omitempty"`
	OverrideUserInfo            *bool                                                                  `json:"overrideUserInfo,omitempty"`
	Pkce                        bool                                                                   `json:"pkce"`
	Scopes                      *[]string                                                              `json:"scopes,omitempty"`
	TokenEndpoint               *string                                                                `json:"tokenEndpoint,omitempty"`
	TokenEndpointAuthentication *client.CreateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication `json:"tokenEndpointAuthentication,omitempty"`
	UserInfoEndpoint            *string                                                                `json:"userInfoEndpoint,omitempty"`
} {
	out := &struct {
		AuthorizationEndpoint *string `json:"authorizationEndpoint,omitempty"`
		ClientId              string  `json:"clientId"`
		ClientSecret          string  `json:"clientSecret"`
		DiscoveryEndpoint     string  `json:"discoveryEndpoint"`
		Issuer                string  `json:"issuer"`
		JwksEndpoint          *string `json:"jwksEndpoint,omitempty"`
		Mapping               *struct {
			Email         *string            `json:"email,omitempty"`
			EmailVerified *string            `json:"emailVerified,omitempty"`
			ExtraFields   *map[string]string `json:"extraFields,omitempty"`
			Id            *string            `json:"id,omitempty"`
			Image         *string            `json:"image,omitempty"`
			Name          *string            `json:"name,omitempty"`
		} `json:"mapping,omitempty"`
		OverrideUserInfo            *bool                                                                  `json:"overrideUserInfo,omitempty"`
		Pkce                        bool                                                                   `json:"pkce"`
		Scopes                      *[]string                                                              `json:"scopes,omitempty"`
		TokenEndpoint               *string                                                                `json:"tokenEndpoint,omitempty"`
		TokenEndpointAuthentication *client.CreateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication `json:"tokenEndpointAuthentication,omitempty"`
		UserInfoEndpoint            *string                                                                `json:"userInfoEndpoint,omitempty"`
	}{
		ClientId:          cfg.ClientID.ValueString(),
		ClientSecret:      cfg.ClientSecret.ValueString(),
		DiscoveryEndpoint: cfg.DiscoveryEndpoint.ValueString(),
		Issuer:            cfg.Issuer.ValueString(),
		Pkce:              cfg.Pkce.ValueBool(),
	}

	if !cfg.AuthorizationEndpoint.IsNull() {
		v := cfg.AuthorizationEndpoint.ValueString()
		out.AuthorizationEndpoint = &v
	}
	if !cfg.TokenEndpoint.IsNull() {
		v := cfg.TokenEndpoint.ValueString()
		out.TokenEndpoint = &v
	}
	if !cfg.UserInfoEndpoint.IsNull() {
		v := cfg.UserInfoEndpoint.ValueString()
		out.UserInfoEndpoint = &v
	}
	if !cfg.JwksEndpoint.IsNull() {
		v := cfg.JwksEndpoint.ValueString()
		out.JwksEndpoint = &v
	}
	if len(cfg.Scopes) > 0 {
		scopes := make([]string, 0, len(cfg.Scopes))
		for _, s := range cfg.Scopes {
			if !s.IsNull() {
				scopes = append(scopes, s.ValueString())
			}
		}
		out.Scopes = &scopes
	}
	if !cfg.OverrideUserInfo.IsNull() {
		v := cfg.OverrideUserInfo.ValueBool()
		out.OverrideUserInfo = &v
	}
	if !cfg.TokenEndpointAuthentication.IsNull() {
		v := cfg.TokenEndpointAuthentication.ValueString()
		cast := client.CreateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication(v)
		out.TokenEndpointAuthentication = &cast
	}
	if cfg.Mapping != nil {
		out.Mapping = expandOidcMapping(cfg.Mapping)
	}

	return out
}

func expandOidcConfigUpdate(cfg OidcConfigModel) *struct {
	AuthorizationEndpoint *string `json:"authorizationEndpoint,omitempty"`
	ClientId              string  `json:"clientId"`
	ClientSecret          string  `json:"clientSecret"`
	DiscoveryEndpoint     string  `json:"discoveryEndpoint"`
	Issuer                string  `json:"issuer"`
	JwksEndpoint          *string `json:"jwksEndpoint,omitempty"`
	Mapping               *struct {
		Email         *string            `json:"email,omitempty"`
		EmailVerified *string            `json:"emailVerified,omitempty"`
		ExtraFields   *map[string]string `json:"extraFields,omitempty"`
		Id            *string            `json:"id,omitempty"`
		Image         *string            `json:"image,omitempty"`
		Name          *string            `json:"name,omitempty"`
	} `json:"mapping,omitempty"`
	OverrideUserInfo            *bool                                                                  `json:"overrideUserInfo,omitempty"`
	Pkce                        bool                                                                   `json:"pkce"`
	Scopes                      *[]string                                                              `json:"scopes,omitempty"`
	TokenEndpoint               *string                                                                `json:"tokenEndpoint,omitempty"`
	TokenEndpointAuthentication *client.UpdateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication `json:"tokenEndpointAuthentication,omitempty"`
	UserInfoEndpoint            *string                                                                `json:"userInfoEndpoint,omitempty"`
} {
	out := &struct {
		AuthorizationEndpoint *string `json:"authorizationEndpoint,omitempty"`
		ClientId              string  `json:"clientId"`
		ClientSecret          string  `json:"clientSecret"`
		DiscoveryEndpoint     string  `json:"discoveryEndpoint"`
		Issuer                string  `json:"issuer"`
		JwksEndpoint          *string `json:"jwksEndpoint,omitempty"`
		Mapping               *struct {
			Email         *string            `json:"email,omitempty"`
			EmailVerified *string            `json:"emailVerified,omitempty"`
			ExtraFields   *map[string]string `json:"extraFields,omitempty"`
			Id            *string            `json:"id,omitempty"`
			Image         *string            `json:"image,omitempty"`
			Name          *string            `json:"name,omitempty"`
		} `json:"mapping,omitempty"`
		OverrideUserInfo            *bool                                                                  `json:"overrideUserInfo,omitempty"`
		Pkce                        bool                                                                   `json:"pkce"`
		Scopes                      *[]string                                                              `json:"scopes,omitempty"`
		TokenEndpoint               *string                                                                `json:"tokenEndpoint,omitempty"`
		TokenEndpointAuthentication *client.UpdateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication `json:"tokenEndpointAuthentication,omitempty"`
		UserInfoEndpoint            *string                                                                `json:"userInfoEndpoint,omitempty"`
	}{
		ClientId:          cfg.ClientID.ValueString(),
		ClientSecret:      cfg.ClientSecret.ValueString(),
		DiscoveryEndpoint: cfg.DiscoveryEndpoint.ValueString(),
		Issuer:            cfg.Issuer.ValueString(),
		Pkce:              cfg.Pkce.ValueBool(),
	}

	if !cfg.AuthorizationEndpoint.IsNull() {
		v := cfg.AuthorizationEndpoint.ValueString()
		out.AuthorizationEndpoint = &v
	}
	if !cfg.TokenEndpoint.IsNull() {
		v := cfg.TokenEndpoint.ValueString()
		out.TokenEndpoint = &v
	}
	if !cfg.UserInfoEndpoint.IsNull() {
		v := cfg.UserInfoEndpoint.ValueString()
		out.UserInfoEndpoint = &v
	}
	if !cfg.JwksEndpoint.IsNull() {
		v := cfg.JwksEndpoint.ValueString()
		out.JwksEndpoint = &v
	}
	if len(cfg.Scopes) > 0 {
		scopes := make([]string, 0, len(cfg.Scopes))
		for _, s := range cfg.Scopes {
			if !s.IsNull() {
				scopes = append(scopes, s.ValueString())
			}
		}
		out.Scopes = &scopes
	}
	if !cfg.OverrideUserInfo.IsNull() {
		v := cfg.OverrideUserInfo.ValueBool()
		out.OverrideUserInfo = &v
	}
	if !cfg.TokenEndpointAuthentication.IsNull() {
		v := cfg.TokenEndpointAuthentication.ValueString()
		cast := client.UpdateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication(v)
		out.TokenEndpointAuthentication = &cast
	}
	if cfg.Mapping != nil {
		out.Mapping = expandOidcMapping(cfg.Mapping)
	}

	return out
}

func expandOidcMapping(mapping *OidcMappingModel) *struct {
	Email         *string            `json:"email,omitempty"`
	EmailVerified *string            `json:"emailVerified,omitempty"`
	ExtraFields   *map[string]string `json:"extraFields,omitempty"`
	Id            *string            `json:"id,omitempty"`
	Image         *string            `json:"image,omitempty"`
	Name          *string            `json:"name,omitempty"`
} {
	if mapping == nil {
		return nil
	}
	out := &struct {
		Email         *string            `json:"email,omitempty"`
		EmailVerified *string            `json:"emailVerified,omitempty"`
		ExtraFields   *map[string]string `json:"extraFields,omitempty"`
		Id            *string            `json:"id,omitempty"`
		Image         *string            `json:"image,omitempty"`
		Name          *string            `json:"name,omitempty"`
	}{}

	if !mapping.Email.IsNull() {
		v := mapping.Email.ValueString()
		out.Email = &v
	}
	if !mapping.EmailVerified.IsNull() {
		v := mapping.EmailVerified.ValueString()
		out.EmailVerified = &v
	}
	// extra_fields is optional; omit from payload to avoid leaking unknowns
	if !mapping.ID.IsNull() {
		v := mapping.ID.ValueString()
		out.Id = &v
	}
	if !mapping.Image.IsNull() {
		v := mapping.Image.ValueString()
		out.Image = &v
	}
	if !mapping.Name.IsNull() {
		v := mapping.Name.ValueString()
		out.Name = &v
	}

	return out
}

func expandSamlConfigCreate(cfg SamlConfigModel) *struct {
	AdditionalParams *map[string]interface{} `json:"additionalParams,omitempty"`
	Audience         *string                 `json:"audience,omitempty"`
	CallbackUrl      string                  `json:"callbackUrl"`
	Cert             string                  `json:"cert"`
	DecryptionPvk    *string                 `json:"decryptionPvk,omitempty"`
	DigestAlgorithm  *string                 `json:"digestAlgorithm,omitempty"`
	EntryPoint       string                  `json:"entryPoint"`
	IdentifierFormat *string                 `json:"identifierFormat,omitempty"`
	IdpMetadata      *struct {
		Cert                 *string `json:"cert,omitempty"`
		EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
		EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
		EntityID             *string `json:"entityID,omitempty"`
		EntityURL            *string `json:"entityURL,omitempty"`
		IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
		Metadata             *string `json:"metadata,omitempty"`
		PrivateKey           *string `json:"privateKey,omitempty"`
		PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
		RedirectURL          *string `json:"redirectURL,omitempty"`
		SingleSignOnService  *[]struct {
			Binding  string `json:"Binding"`
			Location string `json:"Location"`
		} `json:"singleSignOnService,omitempty"`
	} `json:"idpMetadata,omitempty"`
	Issuer  string `json:"issuer"`
	Mapping *struct {
		Email         *string            `json:"email,omitempty"`
		EmailVerified *string            `json:"emailVerified,omitempty"`
		ExtraFields   *map[string]string `json:"extraFields,omitempty"`
		FirstName     *string            `json:"firstName,omitempty"`
		Id            *string            `json:"id,omitempty"`
		LastName      *string            `json:"lastName,omitempty"`
		Name          *string            `json:"name,omitempty"`
	} `json:"mapping,omitempty"`
	PrivateKey         *string `json:"privateKey,omitempty"`
	SignatureAlgorithm *string `json:"signatureAlgorithm,omitempty"`
	SpMetadata         struct {
		Binding              *string `json:"binding,omitempty"`
		EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
		EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
		EntityID             *string `json:"entityID,omitempty"`
		IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
		Metadata             *string `json:"metadata,omitempty"`
		PrivateKey           *string `json:"privateKey,omitempty"`
		PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
	} `json:"spMetadata"`
	WantAssertionsSigned *bool `json:"wantAssertionsSigned,omitempty"`
} {
	out := &struct {
		AdditionalParams *map[string]interface{} `json:"additionalParams,omitempty"`
		Audience         *string                 `json:"audience,omitempty"`
		CallbackUrl      string                  `json:"callbackUrl"`
		Cert             string                  `json:"cert"`
		DecryptionPvk    *string                 `json:"decryptionPvk,omitempty"`
		DigestAlgorithm  *string                 `json:"digestAlgorithm,omitempty"`
		EntryPoint       string                  `json:"entryPoint"`
		IdentifierFormat *string                 `json:"identifierFormat,omitempty"`
		IdpMetadata      *struct {
			Cert                 *string `json:"cert,omitempty"`
			EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
			EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
			EntityID             *string `json:"entityID,omitempty"`
			EntityURL            *string `json:"entityURL,omitempty"`
			IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
			Metadata             *string `json:"metadata,omitempty"`
			PrivateKey           *string `json:"privateKey,omitempty"`
			PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
			RedirectURL          *string `json:"redirectURL,omitempty"`
			SingleSignOnService  *[]struct {
				Binding  string `json:"Binding"`
				Location string `json:"Location"`
			} `json:"singleSignOnService,omitempty"`
		} `json:"idpMetadata,omitempty"`
		Issuer  string `json:"issuer"`
		Mapping *struct {
			Email         *string            `json:"email,omitempty"`
			EmailVerified *string            `json:"emailVerified,omitempty"`
			ExtraFields   *map[string]string `json:"extraFields,omitempty"`
			FirstName     *string            `json:"firstName,omitempty"`
			Id            *string            `json:"id,omitempty"`
			LastName      *string            `json:"lastName,omitempty"`
			Name          *string            `json:"name,omitempty"`
		} `json:"mapping,omitempty"`
		PrivateKey         *string `json:"privateKey,omitempty"`
		SignatureAlgorithm *string `json:"signatureAlgorithm,omitempty"`
		SpMetadata         struct {
			Binding              *string `json:"binding,omitempty"`
			EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
			EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
			EntityID             *string `json:"entityID,omitempty"`
			IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
			Metadata             *string `json:"metadata,omitempty"`
			PrivateKey           *string `json:"privateKey,omitempty"`
			PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
		} `json:"spMetadata"`
		WantAssertionsSigned *bool `json:"wantAssertionsSigned,omitempty"`
	}{
		CallbackUrl: cfg.CallbackURL.ValueString(),
		Cert:        cfg.Cert.ValueString(),
		EntryPoint:  cfg.EntryPoint.ValueString(),
		Issuer:      cfg.Issuer.ValueString(),
	}

	if !cfg.Audience.IsNull() {
		v := cfg.Audience.ValueString()
		out.Audience = &v
	}
	if !cfg.DigestAlgorithm.IsNull() {
		v := cfg.DigestAlgorithm.ValueString()
		out.DigestAlgorithm = &v
	}
	if !cfg.IdentifierFormat.IsNull() {
		v := cfg.IdentifierFormat.ValueString()
		out.IdentifierFormat = &v
	}
	if !cfg.DecryptionPvk.IsNull() {
		v := cfg.DecryptionPvk.ValueString()
		out.DecryptionPvk = &v
	}
	if !cfg.PrivateKey.IsNull() {
		v := cfg.PrivateKey.ValueString()
		out.PrivateKey = &v
	}
	if !cfg.SignatureAlgorithm.IsNull() {
		v := cfg.SignatureAlgorithm.ValueString()
		out.SignatureAlgorithm = &v
	}
	if !cfg.WantAssertionsSigned.IsNull() {
		v := cfg.WantAssertionsSigned.ValueBool()
		out.WantAssertionsSigned = &v
	}
	if cfg.IdpMetadata != nil {
		out.IdpMetadata = expandSamlIdpMetadata(cfg.IdpMetadata)
	}
	if cfg.SpMetadata != nil {
		out.SpMetadata = *expandSamlSpMetadata(cfg.SpMetadata)
	}
	if cfg.Mapping != nil {
		out.Mapping = expandSamlMapping(cfg.Mapping)
	}

	return out
}

func expandSamlConfigUpdate(cfg SamlConfigModel) *struct {
	AdditionalParams *map[string]interface{} `json:"additionalParams,omitempty"`
	Audience         *string                 `json:"audience,omitempty"`
	CallbackUrl      string                  `json:"callbackUrl"`
	Cert             string                  `json:"cert"`
	DecryptionPvk    *string                 `json:"decryptionPvk,omitempty"`
	DigestAlgorithm  *string                 `json:"digestAlgorithm,omitempty"`
	EntryPoint       string                  `json:"entryPoint"`
	IdentifierFormat *string                 `json:"identifierFormat,omitempty"`
	IdpMetadata      *struct {
		Cert                 *string `json:"cert,omitempty"`
		EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
		EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
		EntityID             *string `json:"entityID,omitempty"`
		EntityURL            *string `json:"entityURL,omitempty"`
		IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
		Metadata             *string `json:"metadata,omitempty"`
		PrivateKey           *string `json:"privateKey,omitempty"`
		PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
		RedirectURL          *string `json:"redirectURL,omitempty"`
		SingleSignOnService  *[]struct {
			Binding  string `json:"Binding"`
			Location string `json:"Location"`
		} `json:"singleSignOnService,omitempty"`
	} `json:"idpMetadata,omitempty"`
	Issuer  string `json:"issuer"`
	Mapping *struct {
		Email         *string            `json:"email,omitempty"`
		EmailVerified *string            `json:"emailVerified,omitempty"`
		ExtraFields   *map[string]string `json:"extraFields,omitempty"`
		FirstName     *string            `json:"firstName,omitempty"`
		Id            *string            `json:"id,omitempty"`
		LastName      *string            `json:"lastName,omitempty"`
		Name          *string            `json:"name,omitempty"`
	} `json:"mapping,omitempty"`
	PrivateKey         *string `json:"privateKey,omitempty"`
	SignatureAlgorithm *string `json:"signatureAlgorithm,omitempty"`
	SpMetadata         struct {
		Binding              *string `json:"binding,omitempty"`
		EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
		EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
		EntityID             *string `json:"entityID,omitempty"`
		IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
		Metadata             *string `json:"metadata,omitempty"`
		PrivateKey           *string `json:"privateKey,omitempty"`
		PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
	} `json:"spMetadata"`
	WantAssertionsSigned *bool `json:"wantAssertionsSigned,omitempty"`
} {
	return expandSamlConfigCreate(cfg)
}

func expandSamlIdpMetadata(md *SamlIdpMetadata) *struct {
	Cert                 *string `json:"cert,omitempty"`
	EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
	EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
	EntityID             *string `json:"entityID,omitempty"`
	EntityURL            *string `json:"entityURL,omitempty"`
	IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
	Metadata             *string `json:"metadata,omitempty"`
	PrivateKey           *string `json:"privateKey,omitempty"`
	PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
	RedirectURL          *string `json:"redirectURL,omitempty"`
	SingleSignOnService  *[]struct {
		Binding  string `json:"Binding"`
		Location string `json:"Location"`
	} `json:"singleSignOnService,omitempty"`
} {
	if md == nil {
		return nil
	}
	out := &struct {
		Cert                 *string `json:"cert,omitempty"`
		EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
		EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
		EntityID             *string `json:"entityID,omitempty"`
		EntityURL            *string `json:"entityURL,omitempty"`
		IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
		Metadata             *string `json:"metadata,omitempty"`
		PrivateKey           *string `json:"privateKey,omitempty"`
		PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
		RedirectURL          *string `json:"redirectURL,omitempty"`
		SingleSignOnService  *[]struct {
			Binding  string `json:"Binding"`
			Location string `json:"Location"`
		} `json:"singleSignOnService,omitempty"`
	}{}

	if !md.Cert.IsNull() {
		v := md.Cert.ValueString()
		out.Cert = &v
	}
	if !md.EncPrivateKey.IsNull() {
		v := md.EncPrivateKey.ValueString()
		out.EncPrivateKey = &v
	}
	if !md.EncPrivateKeyPass.IsNull() {
		v := md.EncPrivateKeyPass.ValueString()
		out.EncPrivateKeyPass = &v
	}
	if !md.EntityID.IsNull() {
		v := md.EntityID.ValueString()
		out.EntityID = &v
	}
	if !md.EntityURL.IsNull() {
		v := md.EntityURL.ValueString()
		out.EntityURL = &v
	}
	if !md.IsAssertionEncrypted.IsNull() {
		v := md.IsAssertionEncrypted.ValueBool()
		out.IsAssertionEncrypted = &v
	}
	if !md.Metadata.IsNull() {
		v := md.Metadata.ValueString()
		out.Metadata = &v
	}
	if !md.PrivateKey.IsNull() {
		v := md.PrivateKey.ValueString()
		out.PrivateKey = &v
	}
	if !md.PrivateKeyPass.IsNull() {
		v := md.PrivateKeyPass.ValueString()
		out.PrivateKeyPass = &v
	}
	if !md.RedirectURL.IsNull() {
		v := md.RedirectURL.ValueString()
		out.RedirectURL = &v
	}
	if len(md.SingleSignOnService) > 0 {
		services := make([]struct {
			Binding  string `json:"Binding"`
			Location string `json:"Location"`
		}, 0, len(md.SingleSignOnService))
		for _, svc := range md.SingleSignOnService {
			if svc.Binding.IsNull() || svc.Location.IsNull() {
				continue
			}
			services = append(services, struct {
				Binding  string `json:"Binding"`
				Location string `json:"Location"`
			}{Binding: svc.Binding.ValueString(), Location: svc.Location.ValueString()})
		}
		if len(services) > 0 {
			out.SingleSignOnService = &services
		}
	}

	return out
}

func expandSamlSpMetadata(md *SamlSpMetadata) *struct {
	Binding              *string `json:"binding,omitempty"`
	EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
	EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
	EntityID             *string `json:"entityID,omitempty"`
	IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
	Metadata             *string `json:"metadata,omitempty"`
	PrivateKey           *string `json:"privateKey,omitempty"`
	PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
} {
	if md == nil {
		return nil
	}
	out := &struct {
		Binding              *string `json:"binding,omitempty"`
		EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
		EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
		EntityID             *string `json:"entityID,omitempty"`
		IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
		Metadata             *string `json:"metadata,omitempty"`
		PrivateKey           *string `json:"privateKey,omitempty"`
		PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
	}{}

	if !md.Binding.IsNull() {
		v := md.Binding.ValueString()
		out.Binding = &v
	}
	if !md.EncPrivateKey.IsNull() {
		v := md.EncPrivateKey.ValueString()
		out.EncPrivateKey = &v
	}
	if !md.EncPrivateKeyPass.IsNull() {
		v := md.EncPrivateKeyPass.ValueString()
		out.EncPrivateKeyPass = &v
	}
	if !md.EntityID.IsNull() {
		v := md.EntityID.ValueString()
		out.EntityID = &v
	}
	if !md.IsAssertionEncrypted.IsNull() {
		v := md.IsAssertionEncrypted.ValueBool()
		out.IsAssertionEncrypted = &v
	}
	if !md.Metadata.IsNull() {
		v := md.Metadata.ValueString()
		out.Metadata = &v
	}
	if !md.PrivateKey.IsNull() {
		v := md.PrivateKey.ValueString()
		out.PrivateKey = &v
	}
	if !md.PrivateKeyPass.IsNull() {
		v := md.PrivateKeyPass.ValueString()
		out.PrivateKeyPass = &v
	}

	return out
}

func expandSamlMapping(mapping *SamlMappingModel) *struct {
	Email         *string            `json:"email,omitempty"`
	EmailVerified *string            `json:"emailVerified,omitempty"`
	ExtraFields   *map[string]string `json:"extraFields,omitempty"`
	FirstName     *string            `json:"firstName,omitempty"`
	Id            *string            `json:"id,omitempty"`
	LastName      *string            `json:"lastName,omitempty"`
	Name          *string            `json:"name,omitempty"`
} {
	if mapping == nil {
		return nil
	}
	out := &struct {
		Email         *string            `json:"email,omitempty"`
		EmailVerified *string            `json:"emailVerified,omitempty"`
		ExtraFields   *map[string]string `json:"extraFields,omitempty"`
		FirstName     *string            `json:"firstName,omitempty"`
		Id            *string            `json:"id,omitempty"`
		LastName      *string            `json:"lastName,omitempty"`
		Name          *string            `json:"name,omitempty"`
	}{}

	// ID is required; default to "sub" if not provided
	var id string
	if !mapping.ID.IsNull() {
		id = mapping.ID.ValueString()
	} else {
		id = "sub"
	}
	out.Id = &id

	if !mapping.Email.IsNull() {
		v := mapping.Email.ValueString()
		out.Email = &v
	}
	if !mapping.EmailVerified.IsNull() {
		v := mapping.EmailVerified.ValueString()
		out.EmailVerified = &v
	}
	// extra_fields is optional; omit from payload to avoid leaking unknowns
	if !mapping.FirstName.IsNull() {
		v := mapping.FirstName.ValueString()
		out.FirstName = &v
	}
	if !mapping.LastName.IsNull() {
		v := mapping.LastName.ValueString()
		out.LastName = &v
	}
	if !mapping.Name.IsNull() {
		v := mapping.Name.ValueString()
		out.Name = &v
	}

	return out
}

func expandRoleMapping(mapping *RoleMappingModel) *struct {
	DefaultRole *string `json:"defaultRole,omitempty"`
	Rules       *[]struct {
		Expression string `json:"expression"`
		Role       string `json:"role"`
	} `json:"rules,omitempty"`
	SkipRoleSync *bool `json:"skipRoleSync,omitempty"`
	StrictMode   *bool `json:"strictMode,omitempty"`
} {
	if mapping == nil {
		return nil
	}
	out := &struct {
		DefaultRole *string `json:"defaultRole,omitempty"`
		Rules       *[]struct {
			Expression string `json:"expression"`
			Role       string `json:"role"`
		} `json:"rules,omitempty"`
		SkipRoleSync *bool `json:"skipRoleSync,omitempty"`
		StrictMode   *bool `json:"strictMode,omitempty"`
	}{}

	if !mapping.DefaultRole.IsNull() {
		v := mapping.DefaultRole.ValueString()
		out.DefaultRole = &v
	}
	if len(mapping.Rules) > 0 {
		rules := make([]struct {
			Expression string `json:"expression"`
			Role       string `json:"role"`
		}, 0, len(mapping.Rules))
		for _, rule := range mapping.Rules {
			if rule.Expression.IsNull() || rule.Role.IsNull() {
				continue
			}
			rules = append(rules, struct {
				Expression string `json:"expression"`
				Role       string `json:"role"`
			}{Expression: rule.Expression.ValueString(), Role: rule.Role.ValueString()})
		}
		if len(rules) > 0 {
			out.Rules = &rules
		}
	}
	if !mapping.SkipRoleSync.IsNull() {
		v := mapping.SkipRoleSync.ValueBool()
		out.SkipRoleSync = &v
	}
	if !mapping.StrictMode.IsNull() {
		v := mapping.StrictMode.ValueBool()
		out.StrictMode = &v
	}

	return out
}

func expandTeamSyncConfig(cfg *TeamSyncConfigModel) *struct {
	Enabled          *bool   `json:"enabled,omitempty"`
	GroupsExpression *string `json:"groupsExpression,omitempty"`
} {
	if cfg == nil {
		return nil
	}
	out := &struct {
		Enabled          *bool   `json:"enabled,omitempty"`
		GroupsExpression *string `json:"groupsExpression,omitempty"`
	}{}

	if !cfg.Enabled.IsNull() {
		v := cfg.Enabled.ValueBool()
		out.Enabled = &v
	}
	if !cfg.GroupsExpression.IsNull() {
		v := cfg.GroupsExpression.ValueString()
		out.GroupsExpression = &v
	}

	return out
}

// ssoAPIModel is a normalized view of SSO provider responses to handle differing token auth enum types.

// flattenSsoProvider maps the normalized API model into Terraform state.
// mapSsoProviderToState maps API response to Terraform state.

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

func preserveSensitive(value string, prior *SsoProviderResourceModel, getter func(*SsoProviderResourceModel) types.String) types.String {
	if value != "" {
		return types.StringValue(value)
	}
	if prior != nil {
		prev := getter(prior)
		if !prev.IsNull() {
			return prev
		}
	}
	return types.StringNull()
}
