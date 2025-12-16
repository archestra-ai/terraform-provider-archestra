package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SSOProviderResource{}
var _ resource.ResourceWithImportState = &SSOProviderResource{}

func NewSSOProviderResource() resource.Resource {
	return &SSOProviderResource{}
}

type SSOProviderResource struct {
	client *client.ClientWithResponses
}

type SSOProviderResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Issuer         types.String `tfsdk:"issuer"`
	ProviderID     types.String `tfsdk:"provider_id"`
	Domain         types.String `tfsdk:"domain"`
	OrganizationID types.String `tfsdk:"organization_id"`
	UserID         types.String `tfsdk:"user_id"`
	DomainVerified types.Bool   `tfsdk:"domain_verified"`

	OidcConfig *SSOProviderOIDCConfigModel `tfsdk:"oidc_config"`

	SamlConfig *SSOProviderSAMLConfigModel `tfsdk:"saml_config"`

	RoleMapping *SSOProviderRoleMappingModel `tfsdk:"role_mapping"`

	TeamSyncConfig *SSOProviderTeamSyncConfigModel `tfsdk:"team_sync_config"`
}

type SSOProviderOIDCConfigModel struct {
	AuthorizationEndpoint       types.String                 `tfsdk:"authorization_endpoint"`
	ClientId                    types.String                 `tfsdk:"client_id"`
	ClientSecret                types.String                 `tfsdk:"client_secret"`
	DiscoveryEndpoint           types.String                 `tfsdk:"discovery_endpoint"`
	Issuer                      types.String                 `tfsdk:"issuer"`
	JwksEndpoint                types.String                 `tfsdk:"jwks_endpoint"`
	TokenEndpoint               types.String                 `tfsdk:"token_endpoint"`
	TokenEndpointAuthentication types.String                 `tfsdk:"token_endpoint_authentication"`
	UserInfoEndpoint            types.String                 `tfsdk:"user_info_endpoint"`
	Pkce                        types.Bool                   `tfsdk:"pkce"`
	OverrideUserInfo            types.Bool                   `tfsdk:"override_user_info"`
	Scopes                      types.List                   `tfsdk:"scopes"`
	Mapping                     *SSOProviderOIDCMappingModel `tfsdk:"mapping"`
}

type SSOProviderOIDCMappingModel struct {
	Email         types.String `tfsdk:"email"`
	EmailVerified types.String `tfsdk:"email_verified"`
	ExtraFields   types.Map    `tfsdk:"extra_fields"`
	Id            types.String `tfsdk:"id"`
	Image         types.String `tfsdk:"image"`
	Name          types.String `tfsdk:"name"`
}

type SSOProviderSAMLConfigModel struct {
	Audience             types.String                     `tfsdk:"audience"`
	CallbackUrl          types.String                     `tfsdk:"callback_url"`
	Cert                 types.String                     `tfsdk:"cert"`
	DecryptionPvk        types.String                     `tfsdk:"decryption_pvk"`
	DigestAlgorithm      types.String                     `tfsdk:"digest_algorithm"`
	EntryPoint           types.String                     `tfsdk:"entry_point"`
	IdentifierFormat     types.String                     `tfsdk:"identifier_format"`
	Issuer               types.String                     `tfsdk:"issuer"`
	PrivateKey           types.String                     `tfsdk:"private_key"`
	SignatureAlgorithm   types.String                     `tfsdk:"signature_algorithm"`
	WantAssertionsSigned types.Bool                       `tfsdk:"want_assertions_signed"`
	Mapping              *SSOProviderSAMLMappingModel     `tfsdk:"mapping"`
	IdpMetadata          *SSOProviderSAMLIdpMetadataModel `tfsdk:"idp_metadata"`
	SpMetadata           *SSOProviderSAMLSpMetadataModel  `tfsdk:"sp_metadata"`
	AdditionalParams     types.Map                        `tfsdk:"additional_params"`
}

type SSOProviderSAMLMappingModel struct {
	Email         types.String `tfsdk:"email"`
	EmailVerified types.String `tfsdk:"email_verified"`
	ExtraFields   types.Map    `tfsdk:"extra_fields"`
	FirstName     types.String `tfsdk:"first_name"`
	Id            types.String `tfsdk:"id"`
	LastName      types.String `tfsdk:"last_name"`
	Name          types.String `tfsdk:"name"`
}

type SSOProviderSAMLIdpMetadataModel struct {
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
	SingleSignOnService  types.List   `tfsdk:"single_sign_on_service"`
}

type SSOProviderSAMLSpMetadataModel struct {
	Binding              types.String `tfsdk:"binding"`
	EncPrivateKey        types.String `tfsdk:"enc_private_key"`
	EncPrivateKeyPass    types.String `tfsdk:"enc_private_key_pass"`
	EntityID             types.String `tfsdk:"entity_id"`
	IsAssertionEncrypted types.Bool   `tfsdk:"is_assertion_encrypted"`
	Metadata             types.String `tfsdk:"metadata"`
	PrivateKey           types.String `tfsdk:"private_key"`
	PrivateKeyPass       types.String `tfsdk:"private_key_pass"`
}

type SSOProviderRoleMappingModel struct {
	DefaultRole  types.String `tfsdk:"default_role"`
	Rules        types.List   `tfsdk:"rules"`
	SkipRoleSync types.Bool   `tfsdk:"skip_role_sync"`
	StrictMode   types.Bool   `tfsdk:"strict_mode"`
}

type SSOProviderRoleMappingRuleModel struct {
	Expression types.String `tfsdk:"expression"`
	Role       types.String `tfsdk:"role"`
}

type SSOProviderTeamSyncConfigModel struct {
	Enabled          types.Bool   `tfsdk:"enabled"`
	GroupsExpression types.String `tfsdk:"groups_expression"`
}

func (r *SSOProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_provider"
}

func (r *SSOProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra SSO provider configuration for OIDC or SAML authentication with full configuration support.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SSO provider identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"issuer": schema.StringAttribute{
				MarkdownDescription: "The issuer identifier for SSO provider",
				Required:            true,
			},
			"user_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User ID who created this SSO provider",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "The provider ID (e.g., 'google', 'okta', 'saml')",
				Required:            true,
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID this SSO provider belongs to",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain associated with this SSO provider",
				Required:            true,
			},
			"domain_verified": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether domain has been verified",
			},

			// OIDC Configuration Block
			"oidc_config": schema.SingleNestedAttribute{
				MarkdownDescription: "OIDC configuration for the SSO provider",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"authorization_endpoint": schema.StringAttribute{
						MarkdownDescription: "OIDC authorization endpoint",
						Optional:            true,
					},
					"client_id": schema.StringAttribute{
						MarkdownDescription: "OIDC client ID",
						Optional:            true,
					},
					"client_secret": schema.StringAttribute{
						MarkdownDescription: "OIDC client secret",
						Optional:            true,
						Sensitive:           true,
					},
					"discovery_endpoint": schema.StringAttribute{
						MarkdownDescription: "OIDC discovery endpoint",
						Optional:            true,
					},
					"issuer": schema.StringAttribute{
						MarkdownDescription: "OIDC issuer",
						Optional:            true,
					},
					"jwks_endpoint": schema.StringAttribute{
						MarkdownDescription: "OIDC JWKS endpoint",
						Optional:            true,
					},
					"token_endpoint": schema.StringAttribute{
						MarkdownDescription: "OIDC token endpoint",
						Optional:            true,
					},
					"token_endpoint_authentication": schema.StringAttribute{
						MarkdownDescription: "Token endpoint authentication method (client_secret_basic, client_secret_post)",
						Optional:            true,
					},
					"user_info_endpoint": schema.StringAttribute{
						MarkdownDescription: "OIDC user info endpoint",
						Optional:            true,
					},
					"pkce": schema.BoolAttribute{
						MarkdownDescription: "Enable PKCE flow",
						Optional:            true,
					},
					"override_user_info": schema.BoolAttribute{
						MarkdownDescription: "Override user info from provider",
						Optional:            true,
					},
					"scopes": schema.ListAttribute{
						MarkdownDescription: "OIDC scopes to request",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"mapping": schema.SingleNestedAttribute{
						MarkdownDescription: "OIDC attribute mapping",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"email": schema.StringAttribute{
								MarkdownDescription: "Email attribute mapping",
								Optional:            true,
							},
							"email_verified": schema.StringAttribute{
								MarkdownDescription: "Email verified attribute mapping",
								Optional:            true,
							},
							"extra_fields": schema.MapAttribute{
								MarkdownDescription: "Extra field mappings",
								ElementType:         types.StringType,
								Optional:            true,
							},
							"id": schema.StringAttribute{
								MarkdownDescription: "ID attribute mapping",
								Optional:            true,
							},
							"image": schema.StringAttribute{
								MarkdownDescription: "Image attribute mapping",
								Optional:            true,
							},
							"name": schema.StringAttribute{
								MarkdownDescription: "Name attribute mapping",
								Optional:            true,
							},
						},
					},
				},
			},

			// SAML Configuration Block
			"saml_config": schema.SingleNestedAttribute{
				MarkdownDescription: "SAML configuration for the SSO provider",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"audience": schema.StringAttribute{
						MarkdownDescription: "SAML audience",
						Optional:            true,
					},
					"callback_url": schema.StringAttribute{
						MarkdownDescription: "SAML callback URL",
						Optional:            true,
					},
					"cert": schema.StringAttribute{
						MarkdownDescription: "SAML certificate",
						Optional:            true,
					},
					"decryption_pvk": schema.StringAttribute{
						MarkdownDescription: "SAML decryption private key",
						Optional:            true,
						Sensitive:           true,
					},
					"digest_algorithm": schema.StringAttribute{
						MarkdownDescription: "SAML digest algorithm",
						Optional:            true,
					},
					"entry_point": schema.StringAttribute{
						MarkdownDescription: "SAML entry point",
						Optional:            true,
					},
					"identifier_format": schema.StringAttribute{
						MarkdownDescription: "SAML identifier format",
						Optional:            true,
					},
					"issuer": schema.StringAttribute{
						MarkdownDescription: "SAML issuer",
						Optional:            true,
					},
					"private_key": schema.StringAttribute{
						MarkdownDescription: "SAML private key",
						Optional:            true,
						Sensitive:           true,
					},
					"signature_algorithm": schema.StringAttribute{
						MarkdownDescription: "SAML signature algorithm",
						Optional:            true,
					},
					"want_assertions_signed": schema.BoolAttribute{
						MarkdownDescription: "Require signed assertions",
						Optional:            true,
					},
					"additional_params": schema.MapAttribute{
						MarkdownDescription: "Additional SAML parameters",
						ElementType:         types.StringType,
						Optional:            true,
					},
					"mapping": schema.SingleNestedAttribute{
						MarkdownDescription: "SAML attribute mapping",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"email": schema.StringAttribute{
								MarkdownDescription: "Email attribute mapping",
								Optional:            true,
							},
							"email_verified": schema.StringAttribute{
								MarkdownDescription: "Email verified attribute mapping",
								Optional:            true,
							},
							"extra_fields": schema.MapAttribute{
								MarkdownDescription: "Extra field mappings",
								ElementType:         types.StringType,
								Optional:            true,
							},
							"first_name": schema.StringAttribute{
								MarkdownDescription: "First name attribute mapping",
								Optional:            true,
							},
							"id": schema.StringAttribute{
								MarkdownDescription: "ID attribute mapping",
								Optional:            true,
							},
							"last_name": schema.StringAttribute{
								MarkdownDescription: "Last name attribute mapping",
								Optional:            true,
							},
							"name": schema.StringAttribute{
								MarkdownDescription: "Name attribute mapping",
								Optional:            true,
							},
						},
					},
					"idp_metadata": schema.SingleNestedAttribute{
						MarkdownDescription: "SAML IdP metadata",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"cert": schema.StringAttribute{
								MarkdownDescription: "IdP certificate",
								Optional:            true,
							},
							"enc_private_key": schema.StringAttribute{
								MarkdownDescription: "IdP encryption private key",
								Optional:            true,
								Sensitive:           true,
							},
							"enc_private_key_pass": schema.StringAttribute{
								MarkdownDescription: "IdP encryption private key password",
								Optional:            true,
								Sensitive:           true,
							},
							"entity_id": schema.StringAttribute{
								MarkdownDescription: "IdP entity ID",
								Optional:            true,
							},
							"entity_url": schema.StringAttribute{
								MarkdownDescription: "IdP entity URL",
								Optional:            true,
							},
							"is_assertion_encrypted": schema.BoolAttribute{
								MarkdownDescription: "Whether assertions are encrypted",
								Optional:            true,
							},
							"metadata": schema.StringAttribute{
								MarkdownDescription: "IdP metadata XML",
								Optional:            true,
							},
							"private_key": schema.StringAttribute{
								MarkdownDescription: "IdP private key",
								Optional:            true,
								Sensitive:           true,
							},
							"private_key_pass": schema.StringAttribute{
								MarkdownDescription: "IdP private key password",
								Optional:            true,
								Sensitive:           true,
							},
							"redirect_url": schema.StringAttribute{
								MarkdownDescription: "IdP redirect URL",
								Optional:            true,
							},
							"single_sign_on_service": schema.ListAttribute{
								MarkdownDescription: "IdP SSO service endpoints",
								ElementType:         types.StringType,
								Optional:            true,
							},
						},
					},
					"sp_metadata": schema.SingleNestedAttribute{
						MarkdownDescription: "SAML SP metadata",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"binding": schema.StringAttribute{
								MarkdownDescription: "SP binding",
								Optional:            true,
							},
							"enc_private_key": schema.StringAttribute{
								MarkdownDescription: "SP encryption private key",
								Optional:            true,
								Sensitive:           true,
							},
							"enc_private_key_pass": schema.StringAttribute{
								MarkdownDescription: "SP encryption private key password",
								Optional:            true,
								Sensitive:           true,
							},
							"entity_id": schema.StringAttribute{
								MarkdownDescription: "SP entity ID",
								Optional:            true,
							},
							"is_assertion_encrypted": schema.BoolAttribute{
								MarkdownDescription: "Whether assertions are encrypted",
								Optional:            true,
							},
							"metadata": schema.StringAttribute{
								MarkdownDescription: "SP metadata XML",
								Optional:            true,
							},
							"private_key": schema.StringAttribute{
								MarkdownDescription: "SP private key",
								Optional:            true,
								Sensitive:           true,
							},
							"private_key_pass": schema.StringAttribute{
								MarkdownDescription: "SP private key password",
								Optional:            true,
								Sensitive:           true,
							},
						},
					},
				},
			},

			// Role Mapping Block
			"role_mapping": schema.SingleNestedAttribute{
				MarkdownDescription: "Role mapping configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"default_role": schema.StringAttribute{
						MarkdownDescription: "Default role for users",
						Optional:            true,
					},
					"rules": schema.ListNestedAttribute{
						MarkdownDescription: "Role mapping rules",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"expression": schema.StringAttribute{
									MarkdownDescription: "Expression to match",
									Required:            true,
								},
								"role": schema.StringAttribute{
									MarkdownDescription: "Role to assign",
									Required:            true,
								},
							},
						},
					},
					"skip_role_sync": schema.BoolAttribute{
						MarkdownDescription: "Skip role synchronization",
						Optional:            true,
					},
					"strict_mode": schema.BoolAttribute{
						MarkdownDescription: "Enable strict mode for role mapping",
						Optional:            true,
					},
				},
			},

			// Team Sync Configuration Block
			"team_sync_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Team synchronization configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Enable team synchronization",
						Optional:            true,
					},
					"groups_expression": schema.StringAttribute{
						MarkdownDescription: "Expression for group mapping",
						Optional:            true,
					},
				},
			},
		},
	}
}

func (r *SSOProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *SSOProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SSOProviderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	issuer := plan.Issuer.ValueString()
	providerId := plan.ProviderID.ValueString()
	domain := plan.Domain.ValueString()

	createReq := client.CreateSsoProviderJSONBody{
		Issuer:     issuer,
		ProviderId: providerId,
		Domain:     domain,
	}

	apiResp, err := r.client.CreateSsoProviderWithResponse(ctx, client.CreateSsoProviderJSONRequestBody(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating SSO provider",
			fmt.Sprintf("Could not create SSO provider: %s", err),
		)
		return
	}

	if apiResp.HTTPResponse.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError(
			"Error creating SSO provider",
			fmt.Sprintf("Unexpected status code: %d, body: %s", apiResp.HTTPResponse.StatusCode, string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error creating SSO provider",
			"Empty response body from API",
		)
		return
	}

	orgId := ""
	if apiResp.JSON200.OrganizationId != nil {
		orgId = *apiResp.JSON200.OrganizationId
	}

	userId := ""
	if apiResp.JSON200.UserId != nil {
		userId = *apiResp.JSON200.UserId
	}

	state := SSOProviderResourceModel{
		ID:             types.StringValue(apiResp.JSON200.Id),
		Issuer:         types.StringValue(apiResp.JSON200.Issuer),
		ProviderID:     plan.ProviderID, // Use from plan since not in response
		Domain:         types.StringValue(apiResp.JSON200.Domain),
		OrganizationID: types.StringValue(orgId),
		UserID:         types.StringValue(userId),
		DomainVerified: types.BoolValue(apiResp.JSON200.DomainVerified != nil && *apiResp.JSON200.DomainVerified),
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *SSOProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SSOProviderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API to get SSO provider
	apiResp, err := r.client.GetSsoProviderWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading SSO provider",
			fmt.Sprintf("Could not read SSO provider: %s", err),
		)
		return
	}

	if apiResp.HTTPResponse.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
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

	orgId := ""
	if apiResp.JSON200.OrganizationId != nil {
		orgId = *apiResp.JSON200.OrganizationId
	}

	userId := ""
	if apiResp.JSON200.UserId != nil {
		userId = *apiResp.JSON200.UserId
	}

	updatedState := SSOProviderResourceModel{
		ID:             types.StringValue(apiResp.JSON200.Id),
		Issuer:         types.StringValue(apiResp.JSON200.Issuer),
		ProviderID:     state.ProviderID, // Preserve from state
		Domain:         types.StringValue(apiResp.JSON200.Domain),
		OrganizationID: types.StringValue(orgId),
		UserID:         types.StringValue(userId),
		DomainVerified: types.BoolValue(apiResp.JSON200.DomainVerified != nil && *apiResp.JSON200.DomainVerified),
	}

	// Set state
	diags = resp.State.Set(ctx, &updatedState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *SSOProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SSOProviderResourceModel
	var state SSOProviderResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)

	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert Terraform model to API request
	issuer := plan.Issuer.ValueString()
	providerId := plan.ProviderID.ValueString()
	domain := plan.Domain.ValueString()

	updateReq := client.UpdateSsoProviderJSONBody{
		Issuer:     &issuer,
		ProviderId: &providerId,
		Domain:     &domain,
	}

	// Call API to update SSO provider
	apiResp, err := r.client.UpdateSsoProviderWithResponse(ctx, state.ID.ValueString(), client.UpdateSsoProviderJSONRequestBody(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating SSO provider",
			fmt.Sprintf("Could not update SSO provider: %s", err),
		)
		return
	}

	if apiResp.HTTPResponse.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error updating SSO provider",
			fmt.Sprintf("Unexpected status code: %d, body: %s", apiResp.HTTPResponse.StatusCode, string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error updating SSO provider",
			"Empty response body from API",
		)
		return
	}

	// Convert API response to Terraform model
	orgId := ""
	if apiResp.JSON200.OrganizationId != nil {
		orgId = *apiResp.JSON200.OrganizationId
	}

	userId := ""
	if apiResp.JSON200.UserId != nil {
		userId = *apiResp.JSON200.UserId
	}

	updatedState := SSOProviderResourceModel{
		ID:             types.StringValue(apiResp.JSON200.Id),
		Issuer:         types.StringValue(apiResp.JSON200.Issuer),
		ProviderID:     plan.ProviderID, // Use from plan since not in response
		Domain:         types.StringValue(apiResp.JSON200.Domain),
		OrganizationID: types.StringValue(orgId),
		UserID:         types.StringValue(userId),
		DomainVerified: types.BoolValue(apiResp.JSON200.DomainVerified != nil && *apiResp.JSON200.DomainVerified),
	}

	// Set state
	diags = resp.State.Set(ctx, &updatedState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *SSOProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SSOProviderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteSsoProviderWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting SSO provider",
			fmt.Sprintf("Could not delete SSO provider: %s", err),
		)
		return
	}

	if apiResp.HTTPResponse.StatusCode != http.StatusNoContent && apiResp.HTTPResponse.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error deleting SSO provider",
			fmt.Sprintf("Unexpected status code: %d, body: %s", apiResp.HTTPResponse.StatusCode, string(apiResp.Body)),
		)
		return
	}
}

func (r *SSOProviderResource) modelToCreateAPIRequest(plan *SSOProviderResourceModel) *client.CreateSsoProviderJSONBody {
	req := &client.CreateSsoProviderJSONBody{
		Domain:     plan.Domain.ValueString(),
		Issuer:     plan.Issuer.ValueString(),
		ProviderId: plan.ProviderID.ValueString(),
	}

	if plan.OidcConfig != nil {
		req.OidcConfig = r.modelToOIDCConfigCreate(plan.OidcConfig)
	}

	if plan.SamlConfig != nil {
		req.SamlConfig = r.modelToSAMLConfigCreate(plan.SamlConfig)
	}

	if plan.RoleMapping != nil {
		req.RoleMapping = r.modelToRoleMappingCreate(plan.RoleMapping)
	}

	if plan.TeamSyncConfig != nil {
		req.TeamSyncConfig = r.modelToTeamSyncConfigCreate(plan.TeamSyncConfig)
	}

	return req
}

func (r *SSOProviderResource) modelToUpdateAPIRequest(plan *SSOProviderResourceModel) *client.UpdateSsoProviderJSONBody {
	domain := plan.Domain.ValueString()
	issuer := plan.Issuer.ValueString()
	providerId := plan.ProviderID.ValueString()

	req := &client.UpdateSsoProviderJSONBody{
		Domain:     &domain,
		Issuer:     &issuer,
		ProviderId: &providerId,
	}

	// Convert OIDC config
	if plan.OidcConfig != nil {
		req.OidcConfig = r.modelToOIDCConfigUpdate(plan.OidcConfig)
	}

	// Convert SAML config
	if plan.SamlConfig != nil {
		req.SamlConfig = r.modelToSAMLConfigUpdate(plan.SamlConfig)
	}

	// Convert role mapping
	if plan.RoleMapping != nil {
		req.RoleMapping = r.modelToRoleMappingUpdate(plan.RoleMapping)
	}

	// Convert team sync config
	if plan.TeamSyncConfig != nil {
		req.TeamSyncConfig = r.modelToTeamSyncConfigUpdate(plan.TeamSyncConfig)
	}

	return req
}

func (r *SSOProviderResource) modelToOIDCConfigCreate(model *SSOProviderOIDCConfigModel) *client.CreateSsoProviderJSONBodyOidcConfig {
	if model == nil {
		return nil
	}

	config := &client.CreateSsoProviderJSONBodyOidcConfig{
		ClientId:          model.ClientId.ValueString(),
		ClientSecret:      model.ClientSecret.ValueString(),
		DiscoveryEndpoint: model.DiscoveryEndpoint.ValueString(),
		Issuer:            model.Issuer.ValueString(),
		Pkce:              model.Pkce.ValueBool(),
	}

	if !model.AuthorizationEndpoint.IsNull() {
		authEndpoint := model.AuthorizationEndpoint.ValueString()
		config.AuthorizationEndpoint = &authEndpoint
	}

	if !model.JwksEndpoint.IsNull() {
		jwksEndpoint := model.JwksEndpoint.ValueString()
		config.JwksEndpoint = &jwksEndpoint
	}

	if !model.TokenEndpoint.IsNull() {
		tokenEndpoint := model.TokenEndpoint.ValueString()
		config.TokenEndpoint = &tokenEndpoint
	}

	if !model.TokenEndpointAuthentication.IsNull() {
		auth := model.TokenEndpointAuthentication.ValueString()
		tokenAuth := client.CreateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication(auth)
		config.TokenEndpointAuthentication = &tokenAuth
	}

	if !model.UserInfoEndpoint.IsNull() {
		userInfoEndpoint := model.UserInfoEndpoint.ValueString()
		config.UserInfoEndpoint = &userInfoEndpoint
	}

	if !model.OverrideUserInfo.IsNull() {
		override := model.OverrideUserInfo.ValueBool()
		config.OverrideUserInfo = &override
	}

	if !model.Scopes.IsNull() && len(model.Scopes.Elements()) > 0 {
		var scopes []string
		model.Scopes.ElementsAs(context.Background(), &scopes, false)
		config.Scopes = &scopes
	}

	if model.Mapping != nil {
		config.Mapping = r.modelToOIDCMappingCreate(model.Mapping)
	}

	return config
}

func (r *SSOProviderResource) modelToOIDCConfigUpdate(model *SSOProviderOIDCConfigModel) *client.UpdateSsoProviderJSONBodyOidcConfig {
	if model == nil {
		return nil
	}

	config := &client.UpdateSsoProviderJSONBodyOidcConfig{
		ClientId:          model.ClientId.ValueString(),
		ClientSecret:      model.ClientSecret.ValueString(),
		DiscoveryEndpoint: model.DiscoveryEndpoint.ValueString(),
		Issuer:            model.Issuer.ValueString(),
		Pkce:              model.Pkce.ValueBool(),
	}

	if !model.AuthorizationEndpoint.IsNull() {
		authEndpoint := model.AuthorizationEndpoint.ValueString()
		config.AuthorizationEndpoint = &authEndpoint
	}

	if !model.JwksEndpoint.IsNull() {
		jwksEndpoint := model.JwksEndpoint.ValueString()
		config.JwksEndpoint = &jwksEndpoint
	}

	if !model.TokenEndpoint.IsNull() {
		tokenEndpoint := model.TokenEndpoint.ValueString()
		config.TokenEndpoint = &tokenEndpoint
	}

	if !model.TokenEndpointAuthentication.IsNull() {
		auth := model.TokenEndpointAuthentication.ValueString()
		tokenAuth := client.UpdateSsoProviderJSONBodyOidcConfigTokenEndpointAuthentication(auth)
		config.TokenEndpointAuthentication = &tokenAuth
	}

	if !model.UserInfoEndpoint.IsNull() {
		userInfoEndpoint := model.UserInfoEndpoint.ValueString()
		config.UserInfoEndpoint = &userInfoEndpoint
	}

	if !model.OverrideUserInfo.IsNull() {
		override := model.OverrideUserInfo.ValueBool()
		config.OverrideUserInfo = &override
	}

	if !model.Scopes.IsNull() && len(model.Scopes.Elements()) > 0 {
		var scopes []string
		model.Scopes.ElementsAs(context.Background(), &scopes, false)
		config.Scopes = &scopes
	}

	if model.Mapping != nil {
		config.Mapping = r.modelToOIDCMappingUpdate(model.Mapping)
	}

	return config
}

func (r *SSOProviderResource) modelToOIDCMappingCreate(model *SSOProviderOIDCMappingModel) *client.CreateSsoProviderJSONBodyOidcConfigMapping {
	if model == nil {
		return nil
	}

	mapping := &client.CreateSsoProviderJSONBodyOidcConfigMapping{}

	if !model.Email.IsNull() {
		email := model.Email.ValueString()
		mapping.Email = &email
	}

	if !model.EmailVerified.IsNull() {
		emailVerified := model.EmailVerified.ValueString()
		mapping.EmailVerified = &emailVerified
	}

	if !model.ExtraFields.IsNull() && len(model.ExtraFields.Elements()) > 0 {
		var extraFields map[string]string
		model.ExtraFields.ElementsAs(context.Background(), &extraFields, false)
		mapping.ExtraFields = &extraFields
	}

	if !model.Id.IsNull() {
		id := model.Id.ValueString()
		mapping.Id = &id
	}

	if !model.Image.IsNull() {
		image := model.Image.ValueString()
		mapping.Image = &image
	}

	if !model.Name.IsNull() {
		name := model.Name.ValueString()
		mapping.Name = &name
	}

	return mapping
}

func (r *SSOProviderResource) modelToOIDCMappingUpdate(model *SSOProviderOIDCMappingModel) *client.UpdateSsoProviderJSONBodyOidcConfigMapping {
	if model == nil {
		return nil
	}

	mapping := &client.UpdateSsoProviderJSONBodyOidcConfigMapping{}

	if !model.Email.IsNull() {
		email := model.Email.ValueString()
		mapping.Email = &email
	}

	if !model.EmailVerified.IsNull() {
		emailVerified := model.EmailVerified.ValueString()
		mapping.EmailVerified = &emailVerified
	}

	if !model.ExtraFields.IsNull() && len(model.ExtraFields.Elements()) > 0 {
		var extraFields map[string]string
		model.ExtraFields.ElementsAs(context.Background(), &extraFields, false)
		mapping.ExtraFields = &extraFields
	}

	if !model.Id.IsNull() {
		id := model.Id.ValueString()
		mapping.Id = &id
	}

	if !model.Image.IsNull() {
		image := model.Image.ValueString()
		mapping.Image = &image
	}

	if !model.Name.IsNull() {
		name := model.Name.ValueString()
		mapping.Name = &name
	}

	return mapping
}

func (r *SSOProviderResource) modelToSAMLConfigCreate(model *SSOProviderSAMLConfigModel) *client.CreateSsoProviderJSONBodySamlConfig {
	if model == nil {
		return nil
	}

	config := &client.CreateSsoProviderJSONBodySamlConfig{
		CallbackUrl: model.CallbackUrl.ValueString(),
		Cert:        model.Cert.ValueString(),
		EntryPoint:  model.EntryPoint.ValueString(),
		Issuer:      model.Issuer.ValueString(),
	}

	if !model.Audience.IsNull() {
		audience := model.Audience.ValueString()
		config.Audience = &audience
	}

	if !model.DecryptionPvk.IsNull() {
		decryptionPvk := model.DecryptionPvk.ValueString()
		config.DecryptionPvk = &decryptionPvk
	}

	if !model.DigestAlgorithm.IsNull() {
		digestAlgorithm := model.DigestAlgorithm.ValueString()
		config.DigestAlgorithm = &digestAlgorithm
	}

	if !model.IdentifierFormat.IsNull() {
		identifierFormat := model.IdentifierFormat.ValueString()
		config.IdentifierFormat = &identifierFormat
	}

	if !model.PrivateKey.IsNull() {
		privateKey := model.PrivateKey.ValueString()
		config.PrivateKey = &privateKey
	}

	if !model.SignatureAlgorithm.IsNull() {
		signatureAlgorithm := model.SignatureAlgorithm.ValueString()
		config.SignatureAlgorithm = &signatureAlgorithm
	}

	if !model.WantAssertionsSigned.IsNull() {
		wantAssertionsSigned := model.WantAssertionsSigned.ValueBool()
		config.WantAssertionsSigned = &wantAssertionsSigned
	}

	if !model.AdditionalParams.IsNull() && len(model.AdditionalParams.Elements()) > 0 {
		var additionalParams map[string]interface{}
		model.AdditionalParams.ElementsAs(context.Background(), &additionalParams, false)
		config.AdditionalParams = &additionalParams
	}

	if model.Mapping != nil {
		config.Mapping = r.modelToSAMLMappingCreate(model.Mapping)
	}

	if model.IdpMetadata != nil {
		config.IdpMetadata = r.modelToSAMLIdpMetadataCreate(model.IdpMetadata)
	}

	if model.SpMetadata != nil {
		config.SpMetadata = r.modelToSAMLSpMetadataCreate(model.SpMetadata)
	}

	return config
}

func (r *SSOProviderResource) modelToSAMLConfigUpdate(model *SSOProviderSAMLConfigModel) *client.UpdateSsoProviderJSONBodySamlConfig {
	if model == nil {
		return nil
	}

	config := &client.UpdateSsoProviderJSONBodySamlConfig{
		CallbackUrl: model.CallbackUrl.ValueString(),
		Cert:        model.Cert.ValueString(),
		EntryPoint:  model.EntryPoint.ValueString(),
		Issuer:      model.Issuer.ValueString(),
	}

	if !model.Audience.IsNull() {
		audience := model.Audience.ValueString()
		config.Audience = &audience
	}

	if !model.DecryptionPvk.IsNull() {
		decryptionPvk := model.DecryptionPvk.ValueString()
		config.DecryptionPvk = &decryptionPvk
	}

	if !model.DigestAlgorithm.IsNull() {
		digestAlgorithm := model.DigestAlgorithm.ValueString()
		config.DigestAlgorithm = &digestAlgorithm
	}

	if !model.IdentifierFormat.IsNull() {
		identifierFormat := model.IdentifierFormat.ValueString()
		config.IdentifierFormat = &identifierFormat
	}

	if !model.PrivateKey.IsNull() {
		privateKey := model.PrivateKey.ValueString()
		config.PrivateKey = &privateKey
	}

	if !model.SignatureAlgorithm.IsNull() {
		signatureAlgorithm := model.SignatureAlgorithm.ValueString()
		config.SignatureAlgorithm = &signatureAlgorithm
	}

	if !model.WantAssertionsSigned.IsNull() {
		wantAssertionsSigned := model.WantAssertionsSigned.ValueBool()
		config.WantAssertionsSigned = &wantAssertionsSigned
	}

	if !model.AdditionalParams.IsNull() && len(model.AdditionalParams.Elements()) > 0 {
		var additionalParams map[string]interface{}
		model.AdditionalParams.ElementsAs(context.Background(), &additionalParams, false)
		config.AdditionalParams = &additionalParams
	}

	if model.Mapping != nil {
		config.Mapping = r.modelToSAMLMappingUpdate(model.Mapping)
	}

	if model.IdpMetadata != nil {
		config.IdpMetadata = r.modelToSAMLIdpMetadataUpdate(model.IdpMetadata)
	}

	if model.SpMetadata != nil {
		config.SpMetadata = r.modelToSAMLSpMetadataUpdate(model.SpMetadata)
	}

	return config
}

func (r *SSOProviderResource) modelToSAMLMappingCreate(model *SSOProviderSAMLMappingModel) *client.CreateSsoProviderJSONBodySamlConfigMapping {
	if model == nil {
		return nil
	}

	mapping := &client.CreateSsoProviderJSONBodySamlConfigMapping{}

	if !model.Email.IsNull() {
		email := model.Email.ValueString()
		mapping.Email = &email
	}

	if !model.EmailVerified.IsNull() {
		emailVerified := model.EmailVerified.ValueString()
		mapping.EmailVerified = &emailVerified
	}

	if !model.ExtraFields.IsNull() && len(model.ExtraFields.Elements()) > 0 {
		var extraFields map[string]string
		model.ExtraFields.ElementsAs(context.Background(), &extraFields, false)
		mapping.ExtraFields = &extraFields
	}

	if !model.FirstName.IsNull() {
		firstName := model.FirstName.ValueString()
		mapping.FirstName = &firstName
	}

	if !model.Id.IsNull() {
		id := model.Id.ValueString()
		mapping.Id = &id
	}

	if !model.LastName.IsNull() {
		lastName := model.LastName.ValueString()
		mapping.LastName = &lastName
	}

	if !model.Name.IsNull() {
		name := model.Name.ValueString()
		mapping.Name = &name
	}

	return mapping
}

func (r *SSOProviderResource) modelToSAMLMappingUpdate(model *SSOProviderSAMLMappingModel) *client.UpdateSsoProviderJSONBodySamlConfigMapping {
	if model == nil {
		return nil
	}

	mapping := &client.UpdateSsoProviderJSONBodySamlConfigMapping{}

	if !model.Email.IsNull() {
		email := model.Email.ValueString()
		mapping.Email = &email
	}

	if !model.EmailVerified.IsNull() {
		emailVerified := model.EmailVerified.ValueString()
		mapping.EmailVerified = &emailVerified
	}

	if !model.ExtraFields.IsNull() && len(model.ExtraFields.Elements()) > 0 {
		var extraFields map[string]string
		model.ExtraFields.ElementsAs(context.Background(), &extraFields, false)
		mapping.ExtraFields = &extraFields
	}

	if !model.FirstName.IsNull() {
		firstName := model.FirstName.ValueString()
		mapping.FirstName = &firstName
	}

	if !model.Id.IsNull() {
		id := model.Id.ValueString()
		mapping.Id = &id
	}

	if !model.LastName.IsNull() {
		lastName := model.LastName.ValueString()
		mapping.LastName = &lastName
	}

	if !model.Name.IsNull() {
		name := model.Name.ValueString()
		mapping.Name = &name
	}

	return mapping
}

func (r *SSOProviderResource) modelToSAMLIdpMetadataCreate(model *SSOProviderSAMLIdpMetadataModel) *client.CreateSsoProviderJSONBodySamlConfigIdpMetadata {
	if model == nil {
		return nil
	}

	metadata := &client.CreateSsoProviderJSONBodySamlConfigIdpMetadata{}

	if !model.Cert.IsNull() {
		cert := model.Cert.ValueString()
		metadata.Cert = &cert
	}

	if !model.EncPrivateKey.IsNull() {
		encPrivateKey := model.EncPrivateKey.ValueString()
		metadata.EncPrivateKey = &encPrivateKey
	}

	if !model.EncPrivateKeyPass.IsNull() {
		encPrivateKeyPass := model.EncPrivateKeyPass.ValueString()
		metadata.EncPrivateKeyPass = &encPrivateKeyPass
	}

	if !model.EntityID.IsNull() {
		entityID := model.EntityID.ValueString()
		metadata.EntityID = &entityID
	}

	if !model.EntityURL.IsNull() {
		entityURL := model.EntityURL.ValueString()
		metadata.EntityURL = &entityURL
	}

	if !model.IsAssertionEncrypted.IsNull() {
		isAssertionEncrypted := model.IsAssertionEncrypted.ValueBool()
		metadata.IsAssertionEncrypted = &isAssertionEncrypted
	}

	if !model.Metadata.IsNull() {
		metadataStr := model.Metadata.ValueString()
		metadata.Metadata = &metadataStr
	}

	if !model.PrivateKey.IsNull() {
		privateKey := model.PrivateKey.ValueString()
		metadata.PrivateKey = &privateKey
	}

	if !model.PrivateKeyPass.IsNull() {
		privateKeyPass := model.PrivateKeyPass.ValueString()
		metadata.PrivateKeyPass = &privateKeyPass
	}

	if !model.RedirectURL.IsNull() {
		redirectURL := model.RedirectURL.ValueString()
		metadata.RedirectURL = &redirectURL
	}

	if !model.SingleSignOnService.IsNull() && len(model.SingleSignOnService.Elements()) > 0 {
		var ssoServices []string
		model.SingleSignOnService.ElementsAs(context.Background(), &ssoServices, false)
		// Convert to proper struct format - this is simplified
		// In a real implementation, you'd need to parse the JSON strings into the proper struct
	}

	return metadata
}

func (r *SSOProviderResource) modelToSAMLIdpMetadataUpdate(model *SSOProviderSAMLIdpMetadataModel) *client.UpdateSsoProviderJSONBodySamlConfigIdpMetadata {
	if model == nil {
		return nil
	}

	metadata := &client.UpdateSsoProviderJSONBodySamlConfigIdpMetadata{}

	if !model.Cert.IsNull() {
		cert := model.Cert.ValueString()
		metadata.Cert = &cert
	}

	if !model.EncPrivateKey.IsNull() {
		encPrivateKey := model.EncPrivateKey.ValueString()
		metadata.EncPrivateKey = &encPrivateKey
	}

	if !model.EncPrivateKeyPass.IsNull() {
		encPrivateKeyPass := model.EncPrivateKeyPass.ValueString()
		metadata.EncPrivateKeyPass = &encPrivateKeyPass
	}

	if !model.EntityID.IsNull() {
		entityID := model.EntityID.ValueString()
		metadata.EntityID = &entityID
	}

	if !model.EntityURL.IsNull() {
		entityURL := model.EntityURL.ValueString()
		metadata.EntityURL = &entityURL
	}

	if !model.IsAssertionEncrypted.IsNull() {
		isAssertionEncrypted := model.IsAssertionEncrypted.ValueBool()
		metadata.IsAssertionEncrypted = &isAssertionEncrypted
	}

	if !model.Metadata.IsNull() {
		metadataStr := model.Metadata.ValueString()
		metadata.Metadata = &metadataStr
	}

	if !model.PrivateKey.IsNull() {
		privateKey := model.PrivateKey.ValueString()
		metadata.PrivateKey = &privateKey
	}

	if !model.PrivateKeyPass.IsNull() {
		privateKeyPass := model.PrivateKeyPass.ValueString()
		metadata.PrivateKeyPass = &privateKeyPass
	}

	if !model.RedirectURL.IsNull() {
		redirectURL := model.RedirectURL.ValueString()
		metadata.RedirectURL = &redirectURL
	}

	return metadata
}

func (r *SSOProviderResource) modelToSAMLSpMetadataCreate(model *SSOProviderSAMLSpMetadataModel) client.CreateSsoProviderJSONBodySamlConfigSpMetadata {
	if model == nil {
		return client.CreateSsoProviderJSONBodySamlConfigSpMetadata{}
	}

	metadata := client.CreateSsoProviderJSONBodySamlConfigSpMetadata{}

	if !model.Binding.IsNull() {
		binding := model.Binding.ValueString()
		metadata.Binding = &binding
	}

	if !model.EncPrivateKey.IsNull() {
		encPrivateKey := model.EncPrivateKey.ValueString()
		metadata.EncPrivateKey = &encPrivateKey
	}

	if !model.EncPrivateKeyPass.IsNull() {
		encPrivateKeyPass := model.EncPrivateKeyPass.ValueString()
		metadata.EncPrivateKeyPass = &encPrivateKeyPass
	}

	if !model.EntityID.IsNull() {
		entityID := model.EntityID.ValueString()
		metadata.EntityID = &entityID
	}

	if !model.IsAssertionEncrypted.IsNull() {
		isAssertionEncrypted := model.IsAssertionEncrypted.ValueBool()
		metadata.IsAssertionEncrypted = &isAssertionEncrypted
	}

	if !model.Metadata.IsNull() {
		metadataStr := model.Metadata.ValueString()
		metadata.Metadata = &metadataStr
	}

	if !model.PrivateKey.IsNull() {
		privateKey := model.PrivateKey.ValueString()
		metadata.PrivateKey = &privateKey
	}

	if !model.PrivateKeyPass.IsNull() {
		privateKeyPass := model.PrivateKeyPass.ValueString()
		metadata.PrivateKeyPass = &privateKeyPass
	}

	return metadata
}

func (r *SSOProviderResource) modelToSAMLSpMetadataUpdate(model *SSOProviderSAMLSpMetadataModel) *client.UpdateSsoProviderJSONBodySamlConfigSpMetadata {
	if model == nil {
		return nil
	}

	metadata := &client.UpdateSsoProviderJSONBodySamlConfigSpMetadata{}

	if !model.Binding.IsNull() {
		binding := model.Binding.ValueString()
		metadata.Binding = &binding
	}

	if !model.EncPrivateKey.IsNull() {
		encPrivateKey := model.EncPrivateKey.ValueString()
		metadata.EncPrivateKey = &encPrivateKey
	}

	if !model.EncPrivateKeyPass.IsNull() {
		encPrivateKeyPass := model.EncPrivateKeyPass.ValueString()
		metadata.EncPrivateKeyPass = &encPrivateKeyPass
	}

	if !model.EntityID.IsNull() {
		entityID := model.EntityID.ValueString()
		metadata.EntityID = &entityID
	}

	if !model.IsAssertionEncrypted.IsNull() {
		isAssertionEncrypted := model.IsAssertionEncrypted.ValueBool()
		metadata.IsAssertionEncrypted = &isAssertionEncrypted
	}

	if !model.Metadata.IsNull() {
		metadataStr := model.Metadata.ValueString()
		metadata.Metadata = &metadataStr
	}

	if !model.PrivateKey.IsNull() {
		privateKey := model.PrivateKey.ValueString()
		metadata.PrivateKey = &privateKey
	}

	if !model.PrivateKeyPass.IsNull() {
		privateKeyPass := model.PrivateKeyPass.ValueString()
		metadata.PrivateKeyPass = &privateKeyPass
	}

	return metadata
}

func (r *SSOProviderResource) modelToRoleMappingCreate(model *SSOProviderRoleMappingModel) *client.CreateSsoProviderJSONBodyRoleMapping {
	if model == nil {
		return nil
	}

	roleMapping := &client.CreateSsoProviderJSONBodyRoleMapping{}

	if !model.DefaultRole.IsNull() {
		defaultRole := model.DefaultRole.ValueString()
		roleMapping.DefaultRole = &defaultRole
	}

	if !model.Rules.IsNull() && len(model.Rules.Elements()) > 0 {
		var rules []SSOProviderRoleMappingRuleModel
		model.Rules.ElementsAs(context.Background(), &rules, false)

		apiRules := make([]client.CreateSsoProviderJSONBodyRoleMappingRules, len(rules))
		for i, rule := range rules {
			apiRules[i] = client.CreateSsoProviderJSONBodyRoleMappingRules{
				Expression: rule.Expression.ValueString(),
				Role:       rule.Role.ValueString(),
			}
		}
		roleMapping.Rules = &apiRules
	}

	if !model.SkipRoleSync.IsNull() {
		skipRoleSync := model.SkipRoleSync.ValueBool()
		roleMapping.SkipRoleSync = &skipRoleSync
	}

	if !model.StrictMode.IsNull() {
		strictMode := model.StrictMode.ValueBool()
		roleMapping.StrictMode = &strictMode
	}

	return roleMapping
}

func (r *SSOProviderResource) modelToRoleMappingUpdate(model *SSOProviderRoleMappingModel) *client.UpdateSsoProviderJSONBodyRoleMapping {
	if model == nil {
		return nil
	}

	roleMapping := &client.UpdateSsoProviderJSONBodyRoleMapping{}

	if !model.DefaultRole.IsNull() {
		defaultRole := model.DefaultRole.ValueString()
		roleMapping.DefaultRole = &defaultRole
	}

	if !model.Rules.IsNull() && len(model.Rules.Elements()) > 0 {
		var rules []SSOProviderRoleMappingRuleModel
		model.Rules.ElementsAs(context.Background(), &rules, false)

		apiRules := make([]client.UpdateSsoProviderJSONBodyRoleMappingRules, len(rules))
		for i, rule := range rules {
			apiRules[i] = client.UpdateSsoProviderJSONBodyRoleMappingRules{
				Expression: rule.Expression.ValueString(),
				Role:       rule.Role.ValueString(),
			}
		}
		roleMapping.Rules = &apiRules
	}

	if !model.SkipRoleSync.IsNull() {
		skipRoleSync := model.SkipRoleSync.ValueBool()
		roleMapping.SkipRoleSync = &skipRoleSync
	}

	if !model.StrictMode.IsNull() {
		strictMode := model.StrictMode.ValueBool()
		roleMapping.StrictMode = &strictMode
	}

	return roleMapping
}

func (r *SSOProviderResource) modelToTeamSyncConfigCreate(model *SSOProviderTeamSyncConfigModel) *client.CreateSsoProviderJSONBodyTeamSyncConfig {
	if model == nil {
		return nil
	}

	teamSyncConfig := &client.CreateSsoProviderJSONBodyTeamSyncConfig{}

	if !model.Enabled.IsNull() {
		enabled := model.Enabled.ValueBool()
		teamSyncConfig.Enabled = &enabled
	}

	if !model.GroupsExpression.IsNull() {
		groupsExpression := model.GroupsExpression.ValueString()
		teamSyncConfig.GroupsExpression = &groupsExpression
	}

	return teamSyncConfig
}

func (r *SSOProviderResource) modelToTeamSyncConfigUpdate(model *SSOProviderTeamSyncConfigModel) *client.UpdateSsoProviderJSONBodyTeamSyncConfig {
	if model == nil {
		return nil
	}

	teamSyncConfig := &client.UpdateSsoProviderJSONBodyTeamSyncConfig{}

	if !model.Enabled.IsNull() {
		enabled := model.Enabled.ValueBool()
		teamSyncConfig.Enabled = &enabled
	}

	if !model.GroupsExpression.IsNull() {
		groupsExpression := model.GroupsExpression.ValueString()
		teamSyncConfig.GroupsExpression = &groupsExpression
	}

	return teamSyncConfig
}

func (r *SSOProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
