package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SsoProviderResource{}
var _ resource.ResourceWithImportState = &SsoProviderResource{}

func NewSSOProviderResource() resource.Resource {
	return &SsoProviderResource{}
}

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
}

type OidcConfigModel struct {
	ClientID              types.String   `tfsdk:"client_id"`
	ClientSecret          types.String   `tfsdk:"client_secret"`
	DiscoveryEndpoint     types.String   `tfsdk:"discovery_endpoint"`
	Issuer                types.String   `tfsdk:"issuer"`
	AuthorizationEndpoint types.String   `tfsdk:"authorization_endpoint"`
	TokenEndpoint         types.String   `tfsdk:"token_endpoint"`
	UserInfoEndpoint      types.String   `tfsdk:"user_info_endpoint"`
	JwksEndpoint          types.String   `tfsdk:"jwks_endpoint"`
	Pkce                  types.Bool     `tfsdk:"pkce"`
	Scopes                []types.String `tfsdk:"scopes"`
	Mapping               *OidcMapping   `tfsdk:"mapping"`
}

type OidcMapping struct {
	Email         types.String `tfsdk:"email"`
	EmailVerified types.String `tfsdk:"email_verified"`
	Id            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Image         types.String `tfsdk:"image"`
}

type SamlConfigModel struct {
	CallbackUrl types.String `tfsdk:"callback_url"`
	Cert        types.String `tfsdk:"cert"`
	EntryPoint  types.String `tfsdk:"entry_point"`
	Issuer      types.String `tfsdk:"issuer"`
}

type RoleMappingModel struct {
	DefaultRole  types.String      `tfsdk:"default_role"`
	SkipRoleSync types.Bool        `tfsdk:"skip_role_sync"`
	StrictMode   types.Bool        `tfsdk:"strict_mode"`
	Rules        []RoleMappingRule `tfsdk:"rules"`
}

type RoleMappingRule struct {
	Role       types.String `tfsdk:"role"`
	Expression types.String `tfsdk:"expression"`
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
		MarkdownDescription: "Manages an Archestra SSO Provider (OIDC or SAML).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-generated identifier for the provider.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the provider (e.g., 'okta', 'google').",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain associated with this provider.",
				Required:            true,
			},
			"domain_verified": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain has been verified.",
				Computed:            true,
			},
			"issuer": schema.StringAttribute{
				MarkdownDescription: "The OIDC issuer URL.",
				Optional:            true,
			},
			"oidc_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for OIDC providers.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"client_id": schema.StringAttribute{
						MarkdownDescription: "The client ID.",
						Required:            true,
					},
					"client_secret": schema.StringAttribute{
						MarkdownDescription: "The client secret.",
						Required:            true,
						Sensitive:           true,
					},
					"discovery_endpoint": schema.StringAttribute{
						MarkdownDescription: "The OIDC discovery endpoint.",
						Required:            true,
					},
					"issuer": schema.StringAttribute{
						MarkdownDescription: "The issuer URL.",
						Required:            true,
					},
					"authorization_endpoint": schema.StringAttribute{
						MarkdownDescription: "The authorization endpoint.",
						Optional:            true,
					},
					"token_endpoint": schema.StringAttribute{
						MarkdownDescription: "The token endpoint.",
						Optional:            true,
					},
					"user_info_endpoint": schema.StringAttribute{
						MarkdownDescription: "The user info endpoint.",
						Optional:            true,
					},
					"jwks_endpoint": schema.StringAttribute{
						MarkdownDescription: "The JWKS endpoint.",
						Optional:            true,
					},
					"pkce": schema.BoolAttribute{
						MarkdownDescription: "Enable PKCE.",
						Optional:            true,
					},
					"scopes": schema.ListAttribute{
						MarkdownDescription: "List of scopes.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"mapping": schema.SingleNestedAttribute{
						MarkdownDescription: "Field mappings.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"email": schema.StringAttribute{
								Optional: true,
							},
							"email_verified": schema.StringAttribute{
								Optional: true,
							},
							"id": schema.StringAttribute{
								Optional: true,
							},
							"name": schema.StringAttribute{
								Optional: true,
							},
							"image": schema.StringAttribute{
								Optional: true,
							},
						},
					},
				},
			},
			"saml_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for SAML providers.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"callback_url": schema.StringAttribute{
						MarkdownDescription: "The callback URL (ACS URL).",
						Required:            true,
					},
					"cert": schema.StringAttribute{
						MarkdownDescription: "The IdP certificate.",
						Required:            true,
					},
					"entry_point": schema.StringAttribute{
						MarkdownDescription: "The SSO entry point.",
						Required:            true,
					},
					"issuer": schema.StringAttribute{
						MarkdownDescription: "The issuer.",
						Optional:            true,
					},
				},
			},
			"role_mapping": schema.SingleNestedAttribute{
				MarkdownDescription: "Role mapping configuration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"default_role": schema.StringAttribute{
						MarkdownDescription: "The default role to assign.",
						Optional:            true,
					},
					"skip_role_sync": schema.BoolAttribute{
						MarkdownDescription: "Skip role synchronization on login.",
						Optional:            true,
					},
					"strict_mode": schema.BoolAttribute{
						MarkdownDescription: "Enable strict mode (deny login if no rule matches).",
						Optional:            true,
					},
					"rules": schema.ListNestedAttribute{
						MarkdownDescription: "Role mapping rules.",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"role": schema.StringAttribute{
									MarkdownDescription: "The role to assign.",
									Required:            true,
								},
								"expression": schema.StringAttribute{
									MarkdownDescription: "Handlebars expression to match.",
									Required:            true,
								},
							},
						},
					},
				},
			},
			"team_sync_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Team synchronization configuration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Enable team synchronization.",
						Optional:            true,
					},
					"groups_expression": schema.StringAttribute{
						MarkdownDescription: "Handlebars template to extract groups.",
						Optional:            true,
					},
				},
			},
		},
	}
}

func (r *SsoProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *SsoProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SsoProviderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build request body
	body := client.CreateSsoProviderJSONBody{
		ProviderId: data.ProviderID.ValueString(),
		Domain:     data.Domain.ValueString(),
	}

	if !data.Issuer.IsNull() {
		body.Issuer = data.Issuer.ValueString()
	}

	// Fallback top-level issuer from protocol config if missing
	if body.Issuer == "" {
		if data.OidcConfig != nil && !data.OidcConfig.Issuer.IsNull() {
			body.Issuer = data.OidcConfig.Issuer.ValueString()
		} else if data.SamlConfig != nil && !data.SamlConfig.Issuer.IsNull() {
			body.Issuer = data.SamlConfig.Issuer.ValueString()
		}
	}

	if data.OidcConfig != nil {
		oidc := data.OidcConfig
		scopes := make([]string, len(oidc.Scopes))
		for i, s := range oidc.Scopes {
			scopes[i] = s.ValueString()
		}

		body.OidcConfig = &struct {
			AuthorizationEndpoint *string "json:\"authorizationEndpoint,omitempty\""
			ClientId              string  "json:\"clientId\""
			ClientSecret          string  "json:\"clientSecret\""
			DiscoveryEndpoint     string  "json:\"discoveryEndpoint\""
			Issuer                string  "json:\"issuer\""
			JwksEndpoint          *string "json:\"jwksEndpoint,omitempty\""
			Mapping               *struct {
				Email         *string            "json:\"email,omitempty\""
				EmailVerified *string            "json:\"emailVerified,omitempty\""
				ExtraFields   *map[string]string "json:\"extraFields,omitempty\""
				Id            *string            "json:\"id,omitempty\""
				Image         *string            "json:\"image,omitempty\""
				Name          *string            "json:\"name,omitempty\""
			} "json:\"mapping,omitempty\""
			OverrideUserInfo            *bool                                                                  "json:\"overrideUserInfo,omitempty\""
			Pkce                        bool                                                                   "json:\"pkce\""
			Scopes                      *[]string                                                              "json:\"scopes,omitempty\""
			TokenEndpoint               *string                                                                "json:\"tokenEndpoint,omitempty\""
			TokenEndpointAuthentication *client.CreateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication "json:\"tokenEndpointAuthentication,omitempty\""
			UserInfoEndpoint            *string                                                                "json:\"userInfoEndpoint,omitempty\""
		}{
			ClientId:          oidc.ClientID.ValueString(),
			ClientSecret:      oidc.ClientSecret.ValueString(),
			DiscoveryEndpoint: oidc.DiscoveryEndpoint.ValueString(),
			Issuer:            oidc.Issuer.ValueString(),
			Pkce:              oidc.Pkce.ValueBool(),
			Scopes:            &scopes,
		}

		if !oidc.AuthorizationEndpoint.IsNull() {
			val := oidc.AuthorizationEndpoint.ValueString()
			body.OidcConfig.AuthorizationEndpoint = &val
		}
		if !oidc.TokenEndpoint.IsNull() {
			val := oidc.TokenEndpoint.ValueString()
			body.OidcConfig.TokenEndpoint = &val
		}
		if !oidc.UserInfoEndpoint.IsNull() {
			val := oidc.UserInfoEndpoint.ValueString()
			body.OidcConfig.UserInfoEndpoint = &val
		}
		if !oidc.JwksEndpoint.IsNull() {
			val := oidc.JwksEndpoint.ValueString()
			body.OidcConfig.JwksEndpoint = &val
		}

		if oidc.Mapping != nil {
			body.OidcConfig.Mapping = &struct {
				Email         *string            "json:\"email,omitempty\""
				EmailVerified *string            "json:\"emailVerified,omitempty\""
				ExtraFields   *map[string]string "json:\"extraFields,omitempty\""
				Id            *string            "json:\"id,omitempty\""
				Image         *string            "json:\"image,omitempty\""
				Name          *string            "json:\"name,omitempty\""
			}{}
			if !oidc.Mapping.Email.IsNull() {
				val := oidc.Mapping.Email.ValueString()
				body.OidcConfig.Mapping.Email = &val
			}
			if !oidc.Mapping.EmailVerified.IsNull() {
				val := oidc.Mapping.EmailVerified.ValueString()
				body.OidcConfig.Mapping.EmailVerified = &val
			}
			if !oidc.Mapping.Id.IsNull() {
				val := oidc.Mapping.Id.ValueString()
				body.OidcConfig.Mapping.Id = &val
			}
			if !oidc.Mapping.Name.IsNull() {
				val := oidc.Mapping.Name.ValueString()
				body.OidcConfig.Mapping.Name = &val
			}
			if !oidc.Mapping.Image.IsNull() {
				val := oidc.Mapping.Image.ValueString()
				body.OidcConfig.Mapping.Image = &val
			}
		}
	}

	if data.SamlConfig != nil {
		saml := data.SamlConfig
		body.SamlConfig = &struct {
			AdditionalParams *map[string]interface{} "json:\"additionalParams,omitempty\""
			Audience         *string                 "json:\"audience,omitempty\""
			CallbackUrl      string                  "json:\"callbackUrl\""
			Cert             string                  "json:\"cert\""
			DecryptionPvk    *string                 "json:\"decryptionPvk,omitempty\""
			DigestAlgorithm  *string                 "json:\"digestAlgorithm,omitempty\""
			EntryPoint       string                  "json:\"entryPoint\""
			IdentifierFormat *string                 "json:\"identifierFormat,omitempty\""
			IdpMetadata      *struct {
				Cert                 *string "json:\"cert,omitempty\""
				EncPrivateKey        *string "json:\"encPrivateKey,omitempty\""
				EncPrivateKeyPass    *string "json:\"encPrivateKeyPass,omitempty\""
				EntityID             *string "json:\"entityID,omitempty\""
				EntityURL            *string "json:\"entityURL,omitempty\""
				IsAssertionEncrypted *bool   "json:\"isAssertionEncrypted,omitempty\""
				Metadata             *string "json:\"metadata,omitempty\""
				PrivateKey           *string "json:\"privateKey,omitempty\""
				PrivateKeyPass       *string "json:\"privateKeyPass,omitempty\""
				RedirectURL          *string "json:\"redirectURL,omitempty\""
				SingleSignOnService  *[]struct {
					Binding  string "json:\"Binding\""
					Location string "json:\"Location\""
				} "json:\"singleSignOnService,omitempty\""
			} "json:\"idpMetadata,omitempty\""
			Issuer  string "json:\"issuer\""
			Mapping *struct {
				Email         *string            "json:\"email,omitempty\""
				EmailVerified *string            "json:\"emailVerified,omitempty\""
				ExtraFields   *map[string]string "json:\"extraFields,omitempty\""
				FirstName     *string            "json:\"firstName,omitempty\""
				Id            *string            "json:\"id,omitempty\""
				LastName      *string            "json:\"lastName,omitempty\""
				Name          *string            "json:\"name,omitempty\""
			} "json:\"mapping,omitempty\""
			PrivateKey         *string "json:\"privateKey,omitempty\""
			SignatureAlgorithm *string "json:\"signatureAlgorithm,omitempty\""
			SpMetadata         struct {
				Binding              *string "json:\"binding,omitempty\""
				EncPrivateKey        *string "json:\"encPrivateKey,omitempty\""
				EncPrivateKeyPass    *string "json:\"encPrivateKeyPass,omitempty\""
				EntityID             *string "json:\"entityID,omitempty\""
				IsAssertionEncrypted *bool   "json:\"isAssertionEncrypted,omitempty\""
				Metadata             *string "json:\"metadata,omitempty\""
				PrivateKey           *string "json:\"privateKey,omitempty\""
				PrivateKeyPass       *string "json:\"privateKeyPass,omitempty\""
			} "json:\"spMetadata\""
			WantAssertionsSigned *bool "json:\"wantAssertionsSigned,omitempty\""
		}{
			CallbackUrl: saml.CallbackUrl.ValueString(),
			Cert:        saml.Cert.ValueString(),
			EntryPoint:  saml.EntryPoint.ValueString(),
		}
		if !saml.Issuer.IsNull() {
			val := saml.Issuer.ValueString()
			body.SamlConfig.Issuer = val
		}
	}

	if data.RoleMapping != nil {
		rm := data.RoleMapping
		rules := make([]struct {
			Expression string "json:\"expression\""
			Role       string "json:\"role\""
		}, len(rm.Rules))
		for i, r := range rm.Rules {
			rules[i] = struct {
				Expression string "json:\"expression\""
				Role       string "json:\"role\""
			}{
				Expression: r.Expression.ValueString(),
				Role:       r.Role.ValueString(),
			}
		}

		body.RoleMapping = &struct {
			DefaultRole *string "json:\"defaultRole,omitempty\""
			Rules       *[]struct {
				Expression string "json:\"expression\""
				Role       string "json:\"role\""
			} "json:\"rules,omitempty\""
			SkipRoleSync *bool "json:\"skipRoleSync,omitempty\""
			StrictMode   *bool "json:\"strictMode,omitempty\""
		}{
			Rules: &rules,
		}
		if !rm.DefaultRole.IsNull() {
			val := rm.DefaultRole.ValueString()
			body.RoleMapping.DefaultRole = &val
		}
		if !rm.SkipRoleSync.IsNull() {
			val := rm.SkipRoleSync.ValueBool()
			body.RoleMapping.SkipRoleSync = &val
		}
		if !rm.StrictMode.IsNull() {
			val := rm.StrictMode.ValueBool()
			body.RoleMapping.StrictMode = &val
		}
	}

	if data.TeamSyncConfig != nil {
		ts := data.TeamSyncConfig
		body.TeamSyncConfig = &struct {
			Enabled          *bool   "json:\"enabled,omitempty\""
			GroupsExpression *string "json:\"groupsExpression,omitempty\""
		}{}
		if !ts.Enabled.IsNull() {
			val := ts.Enabled.ValueBool()
			body.TeamSyncConfig.Enabled = &val
		}
		if !ts.GroupsExpression.IsNull() {
			val := ts.GroupsExpression.ValueString()
			body.TeamSyncConfig.GroupsExpression = &val
		}
	}

	apiResp, err := r.client.CreateSsoProviderWithResponse(ctx, client.CreateSsoProviderJSONRequestBody(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create SSO provider, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// NOTE: We assume the ID returned by API is what we should use.
	// However, user input 'provider_id' is also important.
	// If API returns a different ID (e.g. UUID) we should probably store it.
	// But let's assume `provider_id` argument is the canonical ID because
	// GetSsoProvider takes a string which is likely this ID.
	// The API response `Id` field might be the same as input `ProviderId`.

	// Update state
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Domain = types.StringValue(apiResp.JSON200.Domain)
	if apiResp.JSON200.DomainVerified != nil {
		data.DomainVerified = types.BoolValue(*apiResp.JSON200.DomainVerified)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SsoProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SsoProviderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Determine which ID to use for the API call
	apiId := data.ProviderID.ValueString()
	if !data.ID.IsNull() {
		apiId = data.ID.ValueString()
	}

	apiResp, err := r.client.GetSsoProviderWithResponse(ctx, apiId)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read SSO provider, got error: %s", err))
		return
	}

	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Update state
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Domain = types.StringValue(apiResp.JSON200.Domain)
	if apiResp.JSON200.DomainVerified != nil {
		data.DomainVerified = types.BoolValue(*apiResp.JSON200.DomainVerified)
	}

	// Map OIDC Config
	if apiResp.JSON200.OidcConfig != nil {
		if data.OidcConfig == nil {
			data.OidcConfig = &OidcConfigModel{}
		}
		data.OidcConfig.ClientID = types.StringValue(apiResp.JSON200.OidcConfig.ClientId)
		data.OidcConfig.ClientSecret = types.StringValue(apiResp.JSON200.OidcConfig.ClientSecret)
		data.OidcConfig.DiscoveryEndpoint = types.StringValue(apiResp.JSON200.OidcConfig.DiscoveryEndpoint)
		data.OidcConfig.Issuer = types.StringValue(apiResp.JSON200.OidcConfig.Issuer)
		data.OidcConfig.Pkce = types.BoolValue(apiResp.JSON200.OidcConfig.Pkce)

		// Map other optional fields
		if apiResp.JSON200.OidcConfig.AuthorizationEndpoint != nil {
			data.OidcConfig.AuthorizationEndpoint = types.StringValue(*apiResp.JSON200.OidcConfig.AuthorizationEndpoint)
		}
		if apiResp.JSON200.OidcConfig.TokenEndpoint != nil {
			data.OidcConfig.TokenEndpoint = types.StringValue(*apiResp.JSON200.OidcConfig.TokenEndpoint)
		}
		if apiResp.JSON200.OidcConfig.UserInfoEndpoint != nil {
			data.OidcConfig.UserInfoEndpoint = types.StringValue(*apiResp.JSON200.OidcConfig.UserInfoEndpoint)
		}
		if apiResp.JSON200.OidcConfig.JwksEndpoint != nil {
			data.OidcConfig.JwksEndpoint = types.StringValue(*apiResp.JSON200.OidcConfig.JwksEndpoint)
		}

		if apiResp.JSON200.OidcConfig.Scopes != nil {
			scopes := make([]types.String, len(*apiResp.JSON200.OidcConfig.Scopes))
			for i, s := range *apiResp.JSON200.OidcConfig.Scopes {
				scopes[i] = types.StringValue(s)
			}
			data.OidcConfig.Scopes = scopes
		}

		if apiResp.JSON200.OidcConfig.Mapping != nil {
			if data.OidcConfig.Mapping == nil {
				data.OidcConfig.Mapping = &OidcMapping{}
			}
			if apiResp.JSON200.OidcConfig.Mapping.Email != nil {
				data.OidcConfig.Mapping.Email = types.StringValue(*apiResp.JSON200.OidcConfig.Mapping.Email)
			}
			if apiResp.JSON200.OidcConfig.Mapping.EmailVerified != nil {
				data.OidcConfig.Mapping.EmailVerified = types.StringValue(*apiResp.JSON200.OidcConfig.Mapping.EmailVerified)
			}
			if apiResp.JSON200.OidcConfig.Mapping.Id != nil {
				data.OidcConfig.Mapping.Id = types.StringValue(*apiResp.JSON200.OidcConfig.Mapping.Id)
			}
			if apiResp.JSON200.OidcConfig.Mapping.Name != nil {
				data.OidcConfig.Mapping.Name = types.StringValue(*apiResp.JSON200.OidcConfig.Mapping.Name)
			}
			if apiResp.JSON200.OidcConfig.Mapping.Image != nil {
				data.OidcConfig.Mapping.Image = types.StringValue(*apiResp.JSON200.OidcConfig.Mapping.Image)
			}
		}
	} else {
		data.OidcConfig = nil
	}

	// Map SAML Config
	if apiResp.JSON200.SamlConfig != nil {
		if data.SamlConfig == nil {
			data.SamlConfig = &SamlConfigModel{}
		}
		data.SamlConfig.CallbackUrl = types.StringValue(apiResp.JSON200.SamlConfig.CallbackUrl)
		data.SamlConfig.Cert = types.StringValue(apiResp.JSON200.SamlConfig.Cert)
		data.SamlConfig.EntryPoint = types.StringValue(apiResp.JSON200.SamlConfig.EntryPoint)
		data.SamlConfig.Issuer = types.StringValue(apiResp.JSON200.SamlConfig.Issuer)
	} else {
		data.SamlConfig = nil
	}

	// Map Role Mapping
	if apiResp.JSON200.RoleMapping != nil {
		if data.RoleMapping == nil {
			data.RoleMapping = &RoleMappingModel{}
		}
		if apiResp.JSON200.RoleMapping.DefaultRole != nil {
			data.RoleMapping.DefaultRole = types.StringValue(*apiResp.JSON200.RoleMapping.DefaultRole)
		}
		if apiResp.JSON200.RoleMapping.SkipRoleSync != nil {
			data.RoleMapping.SkipRoleSync = types.BoolValue(*apiResp.JSON200.RoleMapping.SkipRoleSync)
		}
		if apiResp.JSON200.RoleMapping.StrictMode != nil {
			data.RoleMapping.StrictMode = types.BoolValue(*apiResp.JSON200.RoleMapping.StrictMode)
		}
		if apiResp.JSON200.RoleMapping.Rules != nil {
			rules := make([]RoleMappingRule, len(*apiResp.JSON200.RoleMapping.Rules))
			for i, r := range *apiResp.JSON200.RoleMapping.Rules {
				rules[i] = RoleMappingRule{
					Expression: types.StringValue(r.Expression),
					Role:       types.StringValue(r.Role),
				}
			}
			data.RoleMapping.Rules = rules
		}
	} else {
		data.RoleMapping = nil
	}

	// Map Team Sync Config
	if apiResp.JSON200.TeamSyncConfig != nil {
		if data.TeamSyncConfig == nil {
			data.TeamSyncConfig = &TeamSyncConfigModel{}
		}
		if apiResp.JSON200.TeamSyncConfig.Enabled != nil {
			data.TeamSyncConfig.Enabled = types.BoolValue(*apiResp.JSON200.TeamSyncConfig.Enabled)
		}
		if apiResp.JSON200.TeamSyncConfig.GroupsExpression != nil {
			data.TeamSyncConfig.GroupsExpression = types.StringValue(*apiResp.JSON200.TeamSyncConfig.GroupsExpression)
		}
	} else {
		data.TeamSyncConfig = nil
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SsoProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SsoProviderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Determine which ID to use for the API call
	apiId := data.ID.ValueString()
	if data.ID.IsNull() {
		apiId = data.ProviderID.ValueString()
	}

	// Build request body
	body := client.UpdateSsoProviderJSONBody{}

	if !data.Domain.IsNull() {
		val := data.Domain.ValueString()
		body.Domain = &val
	}
	if !data.Issuer.IsNull() {
		val := data.Issuer.ValueString()
		body.Issuer = &val
	} else {
		// If null, should we nil it out? The API takes *string, so sending nil means no change usually.
		// If we want to unset it, we might need to send empty string if API supports it, or it depends on API behavior.
		// For now, assume nil means no change.
	}

	if data.OidcConfig != nil {
		oidc := data.OidcConfig
		scopes := make([]string, len(oidc.Scopes))
		for i, s := range oidc.Scopes {
			scopes[i] = s.ValueString()
		}

		body.OidcConfig = &struct {
			AuthorizationEndpoint *string "json:\"authorizationEndpoint,omitempty\""
			ClientId              string  "json:\"clientId\""
			ClientSecret          string  "json:\"clientSecret\""
			DiscoveryEndpoint     string  "json:\"discoveryEndpoint\""
			Issuer                string  "json:\"issuer\""
			JwksEndpoint          *string "json:\"jwksEndpoint,omitempty\""
			Mapping               *struct {
				Email         *string            "json:\"email,omitempty\""
				EmailVerified *string            "json:\"emailVerified,omitempty\""
				ExtraFields   *map[string]string "json:\"extraFields,omitempty\""
				Id            *string            "json:\"id,omitempty\""
				Image         *string            "json:\"image,omitempty\""
				Name          *string            "json:\"name,omitempty\""
			} "json:\"mapping,omitempty\""
			OverrideUserInfo            *bool                                                                  "json:\"overrideUserInfo,omitempty\""
			Pkce                        bool                                                                   "json:\"pkce\""
			Scopes                      *[]string                                                              "json:\"scopes,omitempty\""
			TokenEndpoint               *string                                                                "json:\"tokenEndpoint,omitempty\""
			TokenEndpointAuthentication *client.UpdateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication "json:\"tokenEndpointAuthentication,omitempty\""
			UserInfoEndpoint            *string                                                                "json:\"userInfoEndpoint,omitempty\""
		}{
			ClientId:          oidc.ClientID.ValueString(),
			ClientSecret:      oidc.ClientSecret.ValueString(),
			DiscoveryEndpoint: oidc.DiscoveryEndpoint.ValueString(),
			Issuer:            oidc.Issuer.ValueString(),
			Pkce:              oidc.Pkce.ValueBool(),
			Scopes:            &scopes,
		}

		// Ensure top-level issuer is set for the platform if not explicitly in plan
		if body.Issuer == nil {
			val := oidc.Issuer.ValueString()
			body.Issuer = &val
		}

		if !oidc.AuthorizationEndpoint.IsNull() {
			val := oidc.AuthorizationEndpoint.ValueString()
			body.OidcConfig.AuthorizationEndpoint = &val
		}
		if !oidc.TokenEndpoint.IsNull() {
			val := oidc.TokenEndpoint.ValueString()
			body.OidcConfig.TokenEndpoint = &val
		}
		if !oidc.UserInfoEndpoint.IsNull() {
			val := oidc.UserInfoEndpoint.ValueString()
			body.OidcConfig.UserInfoEndpoint = &val
		}
		if !oidc.JwksEndpoint.IsNull() {
			val := oidc.JwksEndpoint.ValueString()
			body.OidcConfig.JwksEndpoint = &val
		}

		if oidc.Mapping != nil {
			body.OidcConfig.Mapping = &struct {
				Email         *string            "json:\"email,omitempty\""
				EmailVerified *string            "json:\"emailVerified,omitempty\""
				ExtraFields   *map[string]string "json:\"extraFields,omitempty\""
				Id            *string            "json:\"id,omitempty\""
				Image         *string            "json:\"image,omitempty\""
				Name          *string            "json:\"name,omitempty\""
			}{}
			if !oidc.Mapping.Email.IsNull() {
				val := oidc.Mapping.Email.ValueString()
				body.OidcConfig.Mapping.Email = &val
			}
			if !oidc.Mapping.EmailVerified.IsNull() {
				val := oidc.Mapping.EmailVerified.ValueString()
				body.OidcConfig.Mapping.EmailVerified = &val
			}
			if !oidc.Mapping.Id.IsNull() {
				val := oidc.Mapping.Id.ValueString()
				body.OidcConfig.Mapping.Id = &val
			}
			if !oidc.Mapping.Name.IsNull() {
				val := oidc.Mapping.Name.ValueString()
				body.OidcConfig.Mapping.Name = &val
			}
			if !oidc.Mapping.Image.IsNull() {
				val := oidc.Mapping.Image.ValueString()
				body.OidcConfig.Mapping.Image = &val
			}
		}
	}

	if data.SamlConfig != nil {
		saml := data.SamlConfig
		body.SamlConfig = &struct {
			AdditionalParams *map[string]interface{} "json:\"additionalParams,omitempty\""
			Audience         *string                 "json:\"audience,omitempty\""
			CallbackUrl      string                  "json:\"callbackUrl\""
			Cert             string                  "json:\"cert\""
			DecryptionPvk    *string                 "json:\"decryptionPvk,omitempty\""
			DigestAlgorithm  *string                 "json:\"digestAlgorithm,omitempty\""
			EntryPoint       string                  "json:\"entryPoint\""
			IdentifierFormat *string                 "json:\"identifierFormat,omitempty\""
			IdpMetadata      *struct {
				Cert                 *string "json:\"cert,omitempty\""
				EncPrivateKey        *string "json:\"encPrivateKey,omitempty\""
				EncPrivateKeyPass    *string "json:\"encPrivateKeyPass,omitempty\""
				EntityID             *string "json:\"entityID,omitempty\""
				EntityURL            *string "json:\"entityURL,omitempty\""
				IsAssertionEncrypted *bool   "json:\"isAssertionEncrypted,omitempty\""
				Metadata             *string "json:\"metadata,omitempty\""
				PrivateKey           *string "json:\"privateKey,omitempty\""
				PrivateKeyPass       *string "json:\"privateKeyPass,omitempty\""
				RedirectURL          *string "json:\"redirectURL,omitempty\""
				SingleSignOnService  *[]struct {
					Binding  string "json:\"Binding\""
					Location string "json:\"Location\""
				} "json:\"singleSignOnService,omitempty\""
			} "json:\"idpMetadata,omitempty\""
			Issuer  string "json:\"issuer\""
			Mapping *struct {
				Email         *string            "json:\"email,omitempty\""
				EmailVerified *string            "json:\"emailVerified,omitempty\""
				ExtraFields   *map[string]string "json:\"extraFields,omitempty\""
				FirstName     *string            "json:\"firstName,omitempty\""
				Id            *string            "json:\"id,omitempty\""
				LastName      *string            "json:\"lastName,omitempty\""
				Name          *string            "json:\"name,omitempty\""
			} "json:\"mapping,omitempty\""
			PrivateKey         *string "json:\"privateKey,omitempty\""
			SignatureAlgorithm *string "json:\"signatureAlgorithm,omitempty\""
			SpMetadata         struct {
				Binding              *string "json:\"binding,omitempty\""
				EncPrivateKey        *string "json:\"encPrivateKey,omitempty\""
				EncPrivateKeyPass    *string "json:\"encPrivateKeyPass,omitempty\""
				EntityID             *string "json:\"entityID,omitempty\""
				IsAssertionEncrypted *bool   "json:\"isAssertionEncrypted,omitempty\""
				Metadata             *string "json:\"metadata,omitempty\""
				PrivateKey           *string "json:\"privateKey,omitempty\""
				PrivateKeyPass       *string "json:\"privateKeyPass,omitempty\""
			} "json:\"spMetadata\""
			WantAssertionsSigned *bool "json:\"wantAssertionsSigned,omitempty\""
		}{
			CallbackUrl: saml.CallbackUrl.ValueString(),
			Cert:        saml.Cert.ValueString(),
			EntryPoint:  saml.EntryPoint.ValueString(),
		}
		if !saml.Issuer.IsNull() {
			val := saml.Issuer.ValueString()
			body.SamlConfig.Issuer = val

			// Ensure top-level issuer is set for the platform if not explicitly in plan
			if body.Issuer == nil {
				body.Issuer = &val
			}
		}
	}

	if data.RoleMapping != nil {
		rm := data.RoleMapping
		rules := make([]struct {
			Expression string "json:\"expression\""
			Role       string "json:\"role\""
		}, len(rm.Rules))
		for i, r := range rm.Rules {
			rules[i] = struct {
				Expression string "json:\"expression\""
				Role       string "json:\"role\""
			}{
				Expression: r.Expression.ValueString(),
				Role:       r.Role.ValueString(),
			}
		}

		body.RoleMapping = &struct {
			DefaultRole *string "json:\"defaultRole,omitempty\""
			Rules       *[]struct {
				Expression string "json:\"expression\""
				Role       string "json:\"role\""
			} "json:\"rules,omitempty\""
			SkipRoleSync *bool "json:\"skipRoleSync,omitempty\""
			StrictMode   *bool "json:\"strictMode,omitempty\""
		}{
			Rules: &rules,
		}
		if !rm.DefaultRole.IsNull() {
			val := rm.DefaultRole.ValueString()
			body.RoleMapping.DefaultRole = &val
		}
		if !rm.SkipRoleSync.IsNull() {
			val := rm.SkipRoleSync.ValueBool()
			body.RoleMapping.SkipRoleSync = &val
		}
		if !rm.StrictMode.IsNull() {
			val := rm.StrictMode.ValueBool()
			body.RoleMapping.StrictMode = &val
		}
	}

	if data.TeamSyncConfig != nil {
		ts := data.TeamSyncConfig
		body.TeamSyncConfig = &struct {
			Enabled          *bool   "json:\"enabled,omitempty\""
			GroupsExpression *string "json:\"groupsExpression,omitempty\""
		}{}
		if !ts.Enabled.IsNull() {
			val := ts.Enabled.ValueBool()
			body.TeamSyncConfig.Enabled = &val
		}
		if !ts.GroupsExpression.IsNull() {
			val := ts.GroupsExpression.ValueString()
			body.TeamSyncConfig.GroupsExpression = &val
		}
	}

	apiResp, err := r.client.UpdateSsoProviderWithResponse(ctx, apiId, client.UpdateSsoProviderJSONRequestBody(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update SSO provider, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Update state
	data.Domain = types.StringValue(apiResp.JSON200.Domain)
	if apiResp.JSON200.DomainVerified != nil {
		data.DomainVerified = types.BoolValue(*apiResp.JSON200.DomainVerified)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SsoProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SsoProviderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Determine which ID to use for the API call
	apiId := data.ProviderID.ValueString()
	if !data.ID.IsNull() {
		apiId = data.ID.ValueString()
	}

	apiResp, err := r.client.DeleteSsoProviderWithResponse(ctx, apiId)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete SSO provider, got error: %s", err))
		return
	}

	// Success is 200 with result
	if apiResp.JSON200 == nil {
		// API might return 404 if already deleted?
		if apiResp.JSON404 != nil {
			return
		}
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *SsoProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("provider_id"), req, resp)
}
