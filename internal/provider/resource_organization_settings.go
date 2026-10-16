package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &OrganizationSettingsResource{}
var _ resource.ResourceWithImportState = &OrganizationSettingsResource{}

func NewOrganizationSettingsResource() resource.Resource {
	return &OrganizationSettingsResource{}
}

type OrganizationSettingsResource struct {
	client *client.ClientWithResponses
}

type OrganizationSettingsResourceModel struct {
	EnvironmentDefaults                      types.Object         `tfsdk:"environment_defaults"`
	DefaultUserLimits                        types.Map            `tfsdk:"default_user_limits"`
	DefaultPermissions                       types.Object         `tfsdk:"default_permissions"`
	OnlineMcpCatalogEnabled                  types.Bool           `tfsdk:"online_mcp_catalog_enabled"`
	OnlineSkillCatalogEnabled                types.Bool           `tfsdk:"online_skill_catalog_enabled"`
	SkillMarketplaceAnonymousAccess          types.Bool           `tfsdk:"skill_marketplace_anonymous_access"`
	AllowToolAutoAssignment                  types.Bool           `tfsdk:"allow_tool_auto_assignment"`
	OcrChatApiKeyId                          types.String         `tfsdk:"ocr_chat_api_key_id"`
	OcrModel                                 types.String         `tfsdk:"ocr_model"`
	KbBm25K1                                 types.Float64        `tfsdk:"kb_bm25_k1"`
	KbBm25B                                  types.Float64        `tfsdk:"kb_bm25_b"`
	KbContextualRetrievalMode                types.String         `tfsdk:"kb_contextual_retrieval_mode"`
	DefaultModelId                           types.String         `tfsdk:"default_model_id"`
	DefaultMemberRole                        types.String         `tfsdk:"default_member_role"`
	OnboardingWizard                         jsontypes.Normalized `tfsdk:"onboarding_wizard"`
	IconLogoDark                             types.String         `tfsdk:"icon_logo_dark"`
	RequireTwoFactor                         types.Bool           `tfsdk:"require_two_factor"`
	SessionMaxAgeSeconds                     types.Int64          `tfsdk:"session_max_age_seconds"`
	OauthAccessTokenLifetimeSeconds          types.Int64          `tfsdk:"oauth_access_token_lifetime_seconds"`
	ConnectionDefaultMcpGatewayId            types.String         `tfsdk:"connection_default_mcp_gateway_id"`
	ConnectionDefaultClientId                types.String         `tfsdk:"connection_default_client_id"`
	ConnectionShownClientIds                 jsontypes.Normalized `tfsdk:"connection_shown_client_ids"`
	ConnectionClientOrder                    jsontypes.Normalized `tfsdk:"connection_client_order"`
	ConnectionBaseUrls                       types.List           `tfsdk:"connection_base_urls"`
	ConnectionClientFutureProviders          jsontypes.Normalized `tfsdk:"connection_client_future_providers"`
	ConnectionClientProviderKeys             jsontypes.Normalized `tfsdk:"connection_client_provider_keys"`
	ConnectionSkillsEnabled                  types.Bool           `tfsdk:"connection_skills_enabled"`
	ConnectionLlmProxyEnabled                types.Bool           `tfsdk:"connection_llm_proxy_enabled"`
	ConnectionPluginsEnabled                 types.Bool           `tfsdk:"connection_plugins_enabled"`
	ConnectionRuntimeHandoffEnabled          types.Bool           `tfsdk:"connection_runtime_handoff_enabled"`
	ConnectionRuntimeHandoffInstructions     types.String         `tfsdk:"connection_runtime_handoff_instructions"`
	ModelProviderOverrides                   jsontypes.Normalized `tfsdk:"model_provider_overrides"`
	MessagingChannelOverrides                jsontypes.Normalized `tfsdk:"messaging_channel_overrides"`
	KnowledgeConnectorOverrides              jsontypes.Normalized `tfsdk:"knowledge_connector_overrides"`
	PopularAgentOverrides                    jsontypes.Normalized `tfsdk:"popular_agent_overrides"`
	DefaultEnvironmentName                   types.String         `tfsdk:"default_environment_name"`
	DefaultEnvironmentNamespace              types.String         `tfsdk:"default_environment_namespace"`
	DefaultEnvironmentDescription            types.String         `tfsdk:"default_environment_description"`
	DefaultNetworkPolicy                     jsontypes.Normalized `tfsdk:"default_network_policy"`
	DefaultEnvironmentValidationRegex        types.String         `tfsdk:"default_environment_validation_regex"`
	DefaultEnvironmentTrustedImageRegistries jsontypes.Normalized `tfsdk:"default_environment_trusted_image_registries"`
	McpIdleHibernationEnabled                types.Bool           `tfsdk:"mcp_idle_hibernation_enabled"`
	AppsHackathonRecorderEnabled             types.Bool           `tfsdk:"apps_hackathon_recorder_enabled"`
	NewAppsDisabledByDefault                 types.Bool           `tfsdk:"new_apps_disabled_by_default"`
	NewAppsLockedByDefault                   types.Bool           `tfsdk:"new_apps_locked_by_default"`
	ID                                       types.String         `tfsdk:"id"`
	Font                                     types.String         `tfsdk:"font"`
	ColorTheme                               types.String         `tfsdk:"color_theme"`
	Logo                                     types.String         `tfsdk:"logo"`
	OnboardingComplete                       types.Bool           `tfsdk:"onboarding_complete"`

	// Appearance settings
	LogoDark                types.String `tfsdk:"logo_dark"`
	Favicon                 types.String `tfsdk:"favicon"`
	IconLogo                types.String `tfsdk:"icon_logo"`
	AppName                 types.String `tfsdk:"app_name"`
	FooterText              types.String `tfsdk:"footer_text"`
	OgDescription           types.String `tfsdk:"og_description"`
	ChatErrorSupportMessage types.String `tfsdk:"chat_error_support_message"`
	ChatPlaceholders        types.List   `tfsdk:"chat_placeholders"`
	ChatLinks               types.List   `tfsdk:"chat_links"`
	AnimateChatPlaceholders types.Bool   `tfsdk:"animate_chat_placeholders"`
	SlimChatErrorUI         types.Bool   `tfsdk:"slim_chat_error_ui"`

	// Security settings
	AllowChatFileUploads types.Bool `tfsdk:"allow_chat_file_uploads"`

	// Agent settings
	DefaultLlmApiKeyId types.String `tfsdk:"default_llm_api_key_id"`
	DefaultAgentId     types.String `tfsdk:"default_agent_id"`

	// MCP settings

	// Knowledge settings
	EmbeddingModel        types.String `tfsdk:"embedding_model"`
	EmbeddingChatApiKeyId types.String `tfsdk:"embedding_chat_api_key_id"`
	RerankerModel         types.String `tfsdk:"reranker_model"`
	RerankerChatApiKeyId  types.String `tfsdk:"reranker_chat_api_key_id"`

	// Read-only org-identity / metadata exposed by the GET endpoint. Updates
	// are routed through better-auth (`/api/auth/organization/*`) which the
	// provider doesn't surface today, so all five are Computed-only and
	// reflect whatever the backend returns at refresh time.
	Name                types.String  `tfsdk:"name"`
	Slug                types.String  `tfsdk:"slug"`
	EmbeddingDimensions types.Float64 `tfsdk:"embedding_dimensions"`
	CreatedAt           types.String  `tfsdk:"created_at"`
	Metadata            types.String  `tfsdk:"metadata"`
}

type ChatLinkModel struct {
	Label types.String `tfsdk:"label"`
	URL   types.String `tfsdk:"url"`
}

func (r *OrganizationSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_settings"
}

func (r *OrganizationSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		MarkdownDescription: "Manages organization settings in Archestra. This is a singleton resource — only one instance can exist per organization.\n\n" +
			"**Lifecycle semantics depend on whether the field has a `Default:`.**\n\n" +
			"- Fields with a documented default (`font`, `color_theme`): omitting the field from your `.tf` resets it to the default on the next apply. To preserve the current backend value, set the field explicitly.\n" +
			"- Fields without a default (most settings — appearance, security, agent, MCP, knowledge): omitting the field is *sticky* — the merge-patch sends nothing for that field and the backend value is preserved. Once a value is set on the backend, you cannot clear it by removing the attribute from HCL; use the platform UI/API directly if you need to wipe a setting.\n\n" +
			"`terraform destroy` only removes this resource from Terraform state; backend settings are never deleted. To stop managing the entire resource without touching the backend, run `terraform state rm archestra_organization_settings.<n>`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"font": schema.StringAttribute{
				MarkdownDescription: "Custom font for the organization UI",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string("lato")),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string("inter"),
						string("jetbrains-mono"),
						string("lato"),
						string("open-sans"),
						string("roboto"),
						string("source-sans-pro"),
					),
				},
			},
			"color_theme": schema.StringAttribute{
				MarkdownDescription: "Color theme for the organization UI",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string("cosmic-night")),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string("amber-minimal"),
						string("boxy-minimalistic"),
						string("bubblegum"),
						string("caffeine"),
						string("catppuccin"),
						string("claude"),
						string("clean-slate"),
						string("cosmic-night"),
						string("doom-64"),
						string("dracula-dark"),
						string("gruvbox-dark"),
						string("mocha-mousse"),
						string("modern-minimal"),
						string("mono"),
						string("monokai-dark"),
						string("moonlight-dark"),
						string("nature"),
						string("neo-brutalism"),
						string("solarized-dark"),
						string("sunset-horizon"),
						string("tangerine"),
						string("twitter"),
						string("vercel"),
						string("vintage-paper"),
					),
				},
			},
			"logo": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded logo image for the organization",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"onboarding_complete": schema.BoolAttribute{
				MarkdownDescription: "Whether organization onboarding is complete. This is a one-way flag — once set to `true`, it cannot be reverted to `false`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},

			// Appearance settings
			"logo_dark": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded dark mode logo image for the organization",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"favicon": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded favicon image for the organization",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"icon_logo": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded icon logo image for the organization",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app_name": schema.StringAttribute{
				MarkdownDescription: "Custom application name displayed in the UI",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"footer_text": schema.StringAttribute{
				MarkdownDescription: "Custom footer text displayed in the UI",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"og_description": schema.StringAttribute{
				MarkdownDescription: "OG meta description for the organization, max 500 characters",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"chat_error_support_message": schema.StringAttribute{
				MarkdownDescription: "Custom error support message displayed in the chat UI",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"chat_placeholders": schema.ListAttribute{
				MarkdownDescription: "Chat placeholder texts displayed in the chat input",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"chat_links": schema.ListNestedAttribute{
				MarkdownDescription: "Chat links displayed in the chat UI",
				Optional:            true,
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label": schema.StringAttribute{
							MarkdownDescription: "Display label for the chat link",
							Required:            true,
						},
						"url": schema.StringAttribute{
							MarkdownDescription: "URL for the chat link",
							Required:            true,
						},
					},
				},
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"animate_chat_placeholders": schema.BoolAttribute{
				MarkdownDescription: "Whether to animate chat placeholders in the UI",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"slim_chat_error_ui": schema.BoolAttribute{
				MarkdownDescription: "When enabled, renders a compact error UI in chat views.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},

			// Security settings
			"allow_chat_file_uploads": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow file uploads in chat",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},

			// Agent settings
			"default_llm_api_key_id": schema.StringAttribute{
				MarkdownDescription: "Default LLM API key ID for the organization",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"default_agent_id": schema.StringAttribute{
				MarkdownDescription: "Default agent (profile) ID for the organization",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// MCP settings

			// Knowledge settings
			"embedding_model": schema.StringAttribute{
				MarkdownDescription: "Embedding model for knowledge base. **Warning: locked after first configuration.** Changing requires dropping embedding config via the API first.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"embedding_chat_api_key_id": schema.StringAttribute{
				MarkdownDescription: "API key ID for the embedding model. **Warning: locked after first configuration.** Changing requires dropping embedding config via the API first.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"reranker_model": schema.StringAttribute{
				MarkdownDescription: "Reranker model for knowledge base",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"reranker_chat_api_key_id": schema.StringAttribute{
				MarkdownDescription: "API key ID for the reranker model",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// Read-only org-identity / metadata. These come back from
			// GET /api/organization but the backend doesn't expose typed
			// update endpoints for them on the settings routes — name/slug
			// are managed by the auth layer at organization creation time
			// and updated through admin tooling outside this resource's
			// surface. Computed-only so users get drift visibility without
			// the provider claiming a write path it doesn't have.
			"name": schema.StringAttribute{
				MarkdownDescription: "Organization display name. Read-only — set at organization creation time and managed by the auth layer; this resource cannot update it.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "Unique URL-safe organization slug. Read-only — set at organization creation time and managed by the auth layer; this resource cannot update it.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"embedding_dimensions": schema.Float64Attribute{
				MarkdownDescription: "Configured embedding model output dimensions. **Deprecated** — the backend is migrating this to `models.embeddingDimensions` (per-model rather than per-org). Exposed here only so existing organizations whose dimensions are still pinned at the org level can read the value via Terraform.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Float64{float64planmodifier.UseStateForUnknown()},
				DeprecationMessage:  "embedding_dimensions is being migrated to per-model storage and will be removed once all organizations are migrated. Read it for now if needed; do not depend on it long-term.",
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp the organization was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "Free-form metadata blob attached to the organization (text; the auth layer typically stores JSON-encoded data here). Read-only on this resource — set by the auth layer.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
	for name, attribute := range organizationDefaultsAttributes() {
		resp.Schema.Attributes[name] = attribute
	}
	resp.Schema.Attributes["default_permissions"] = defaultPermissionsAttribute()
	for name, attribute := range currentOrganizationAttributes() {
		resp.Schema.Attributes[name] = attribute
	}
}

func (r *OrganizationSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrganizationSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationSettingsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Keep the singleton identity recoverable if a later settings write fails.
	var observed OrganizationSettingsResourceModel
	r.readOrganization(ctx, &observed, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &observed)...)
	if resp.Diagnostics.HasError() {
		return
	}
	prior := tftypes.NewValue(req.Plan.Raw.Type(), nil)
	r.applySettings(ctx, req.Plan.Raw, prior, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyDefaults(ctx, req.Plan.Raw, prior, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyDefaultPermissions(ctx, req.Plan.Raw, prior, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readOrganization(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationSettingsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	r.readOrganization(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationSettingsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	r.applySettings(ctx, req.Plan.Raw, req.State.Raw, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyDefaults(ctx, req.Plan.Raw, req.State.Raw, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyDefaultPermissions(ctx, req.Plan.Raw, req.State.Raw, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readOrganization(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Organization settings cannot be deleted via API.
	// Removing from Terraform state only - the organization settings will remain on the server.
}

func (r *OrganizationSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applySettings fans out plan-vs-prior diffs to the six per-domain backend
// endpoints. An endpoint whose merge-patch is empty is skipped — that's the
// whole win over the previous "send every non-null field every time" flow.
//
// CompleteOnboarding is fire-once: only invoked when the plan flips
// onboarding_complete to true and the prior wasn't already true. Backend
// rejects flipping it back to false, so we don't bother diffing the other
// direction.
func (r *OrganizationSettingsResource) applySettings(ctx context.Context, plan, prior tftypes.Value, diags *diag.Diagnostics) {
	endpoints := []struct {
		name string
		spec []AttrSpec
		send func(io.Reader) (int, []byte, error)
	}{
		{"appearance", append(orgSettingsAppearanceSpec, currentOrganizationSpecs("appearance-settings")...), func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateAppearanceSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"security", append(orgSettingsSecuritySpec, currentOrganizationSpecs("security-settings")...), func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateSecuritySettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"agent", append(orgSettingsAgentSpec, currentOrganizationSpecs("agent-settings")...), func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateAgentSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"mcp", append(orgSettingsMcpSpec, currentOrganizationSpecs("mcp-settings")...), func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateMcpSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"knowledge", append(orgSettingsKnowledgeSpec, currentOrganizationSpecs("knowledge-settings")...), func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateKnowledgeSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"skills-settings", currentOrganizationSpecs("skills-settings"), func(body io.Reader) (int, []byte, error) {
			response, err := r.client.UpdateSkillsSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return response.StatusCode(), response.Body, nil
		}},
		{"auth-settings", currentOrganizationSpecs("auth-settings"), func(body io.Reader) (int, []byte, error) {
			response, err := r.client.UpdateAuthSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return response.StatusCode(), response.Body, nil
		}},
		{"connection-settings", currentOrganizationSpecs("connection-settings"), func(body io.Reader) (int, []byte, error) {
			response, err := r.client.UpdateConnectionSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return response.StatusCode(), response.Body, nil
		}},
		{"integration-settings", currentOrganizationSpecs("integration-settings"), func(body io.Reader) (int, []byte, error) {
			response, err := r.client.UpdateIntegrationSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return response.StatusCode(), response.Body, nil
		}},
		{"default-environment", currentOrganizationSpecs("default-environment"), func(body io.Reader) (int, []byte, error) {
			response, err := r.client.UpdateDefaultEnvironmentWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return response.StatusCode(), response.Body, nil
		}},
	}

	for _, ep := range endpoints {
		patch := MergePatch(ctx, plan, prior, ep.spec, diags)
		if diags.HasError() {
			return
		}
		if len(patch) == 0 {
			continue
		}
		LogPatch(ctx, "archestra_organization_settings "+ep.name, patch, ep.spec)

		body, err := json.Marshal(patch)
		if err != nil {
			diags.AddError("Marshal Error", fmt.Sprintf("Unable to marshal %s patch: %s", ep.name, err))
			return
		}
		status, respBody, err := ep.send(bytes.NewReader(body))
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to update %s settings: %s", ep.name, err))
			return
		}
		if status != http.StatusOK {
			diags.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK from %s settings, got status %d: %s", ep.name, status, string(respBody)),
			)
			return
		}
	}

	if shouldCompleteOnboarding(plan, prior) {
		onboardingResp, err := r.client.CompleteOnboardingWithResponse(ctx, client.CompleteOnboardingJSONRequestBody{
			OnboardingComplete: true,
		})
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to complete onboarding: %s", err))
			return
		}
		if onboardingResp.JSON200 == nil {
			diags.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK from CompleteOnboarding, got status %d: %s", onboardingResp.StatusCode(), string(onboardingResp.Body)),
			)
			return
		}
	}
}

// shouldCompleteOnboarding reports whether the plan asks for a fresh transition
// from "not complete" (null or false) to true. The endpoint is one-way; replaying
// it on every Update would be wasted but harmless, but we skip when prior was
// already true to keep refresh diffs clean.
func shouldCompleteOnboarding(plan, prior tftypes.Value) bool {
	planV := lookupOrNull(plan, "onboarding_complete")
	if !planV.IsKnown() || planV.IsNull() {
		return false
	}
	var v bool
	if err := planV.As(&v); err != nil || !v {
		return false
	}
	priorV := lookupOrNull(prior, "onboarding_complete")
	if !priorV.IsKnown() || priorV.IsNull() {
		return true
	}
	var pv bool
	if err := priorV.As(&pv); err != nil {
		return true
	}
	return !pv
}

func (r *OrganizationSettingsResource) readOrganization(ctx context.Context, data *OrganizationSettingsResourceModel, diags *diag.Diagnostics) {
	defer func() {
		if !diags.HasError() {
			r.readDefaults(ctx, data, diags)
			if !diags.HasError() {
				r.readDefaultPermissions(ctx, data, diags)
			}
		}
	}()

	apiResp, err := r.client.GetOrganizationWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read organization settings, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	flattenCurrentOrganizationSettings(data, apiResp.Body, diags)
	if diags.HasError() {
		return
	}
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Font = types.StringValue(string(apiResp.JSON200.CustomFont))
	data.ColorTheme = types.StringValue(string(apiResp.JSON200.Theme))
	data.OnboardingComplete = types.BoolValue(apiResp.JSON200.OnboardingComplete)

	if apiResp.JSON200.Logo != nil {
		data.Logo = types.StringValue(*apiResp.JSON200.Logo)
	} else {
		data.Logo = types.StringNull()
	}

	// Appearance settings
	if apiResp.JSON200.LogoDark != nil {
		data.LogoDark = types.StringValue(*apiResp.JSON200.LogoDark)
	} else {
		data.LogoDark = types.StringNull()
	}
	if apiResp.JSON200.Favicon != nil {
		data.Favicon = types.StringValue(*apiResp.JSON200.Favicon)
	} else {
		data.Favicon = types.StringNull()
	}
	if apiResp.JSON200.IconLogo != nil {
		data.IconLogo = types.StringValue(*apiResp.JSON200.IconLogo)
	} else {
		data.IconLogo = types.StringNull()
	}
	if apiResp.JSON200.AppName != nil {
		data.AppName = types.StringValue(*apiResp.JSON200.AppName)
	} else {
		data.AppName = types.StringNull()
	}
	if apiResp.JSON200.FooterText != nil {
		data.FooterText = types.StringValue(*apiResp.JSON200.FooterText)
	} else {
		data.FooterText = types.StringNull()
	}
	if apiResp.JSON200.OgDescription != nil {
		data.OgDescription = types.StringValue(*apiResp.JSON200.OgDescription)
	} else {
		data.OgDescription = types.StringNull()
	}
	if apiResp.JSON200.ChatErrorSupportMessage != nil {
		data.ChatErrorSupportMessage = types.StringValue(*apiResp.JSON200.ChatErrorSupportMessage)
	} else {
		data.ChatErrorSupportMessage = types.StringNull()
	}
	if apiResp.JSON200.ChatPlaceholders != nil && len(*apiResp.JSON200.ChatPlaceholders) > 0 {
		placeholderValues := make([]attr.Value, len(*apiResp.JSON200.ChatPlaceholders))
		for i, p := range *apiResp.JSON200.ChatPlaceholders {
			placeholderValues[i] = types.StringValue(p)
		}
		data.ChatPlaceholders, _ = types.ListValue(types.StringType, placeholderValues)
	} else {
		data.ChatPlaceholders = types.ListNull(types.StringType)
	}
	if apiResp.JSON200.ChatLinks != nil && len(*apiResp.JSON200.ChatLinks) > 0 {
		chatLinkAttrTypes := map[string]attr.Type{
			"label": types.StringType,
			"url":   types.StringType,
		}
		chatLinkValues := make([]attr.Value, len(*apiResp.JSON200.ChatLinks))
		for i, cl := range *apiResp.JSON200.ChatLinks {
			chatLinkValues[i], _ = types.ObjectValue(chatLinkAttrTypes, map[string]attr.Value{
				"label": types.StringValue(cl.Label),
				"url":   types.StringValue(cl.Url),
			})
		}
		data.ChatLinks, _ = types.ListValue(types.ObjectType{AttrTypes: chatLinkAttrTypes}, chatLinkValues)
	} else {
		data.ChatLinks = types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
			"label": types.StringType,
			"url":   types.StringType,
		}})
	}
	data.AnimateChatPlaceholders = types.BoolValue(apiResp.JSON200.AnimateChatPlaceholders)
	data.SlimChatErrorUI = types.BoolValue(apiResp.JSON200.SlimChatErrorUi)

	// Security settings
	data.AllowChatFileUploads = types.BoolValue(apiResp.JSON200.AllowChatFileUploads)

	// Agent settings
	if apiResp.JSON200.DefaultLlmApiKeyId != nil {
		data.DefaultLlmApiKeyId = types.StringValue(apiResp.JSON200.DefaultLlmApiKeyId.String())
	} else {
		data.DefaultLlmApiKeyId = types.StringNull()
	}
	if apiResp.JSON200.DefaultAgentId != nil {
		data.DefaultAgentId = types.StringValue(apiResp.JSON200.DefaultAgentId.String())
	} else {
		data.DefaultAgentId = types.StringNull()
	}

	// MCP settings

	// Knowledge settings
	if apiResp.JSON200.EmbeddingModel != nil {
		data.EmbeddingModel = types.StringValue(*apiResp.JSON200.EmbeddingModel)
	} else {
		data.EmbeddingModel = types.StringNull()
	}
	if apiResp.JSON200.EmbeddingChatApiKeyId != nil {
		data.EmbeddingChatApiKeyId = types.StringValue(apiResp.JSON200.EmbeddingChatApiKeyId.String())
	} else {
		data.EmbeddingChatApiKeyId = types.StringNull()
	}
	if apiResp.JSON200.RerankerModel != nil {
		data.RerankerModel = types.StringValue(*apiResp.JSON200.RerankerModel)
	} else {
		data.RerankerModel = types.StringNull()
	}
	if apiResp.JSON200.RerankerChatApiKeyId != nil {
		data.RerankerChatApiKeyId = types.StringValue(apiResp.JSON200.RerankerChatApiKeyId.String())
	} else {
		data.RerankerChatApiKeyId = types.StringNull()
	}

	// Read-only org-identity / metadata.
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Slug = types.StringValue(apiResp.JSON200.Slug)
	data.CreatedAt = types.StringValue(apiResp.JSON200.CreatedAt.Format(time.RFC3339))
	if apiResp.JSON200.EmbeddingDimensions != nil {
		data.EmbeddingDimensions = types.Float64Value(float64(*apiResp.JSON200.EmbeddingDimensions))
	} else {
		data.EmbeddingDimensions = types.Float64Null()
	}
	if apiResp.JSON200.Metadata != nil {
		data.Metadata = types.StringValue(*apiResp.JSON200.Metadata)
	} else {
		data.Metadata = types.StringNull()
	}
}

func (r *OrganizationSettingsResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
