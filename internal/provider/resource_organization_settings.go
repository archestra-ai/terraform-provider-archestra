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
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
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
	ID                       types.String `tfsdk:"id"`
	Font                     types.String `tfsdk:"font"`
	ColorTheme               types.String `tfsdk:"color_theme"`
	Logo                     types.String `tfsdk:"logo"`
	LimitCleanupInterval     types.String `tfsdk:"limit_cleanup_interval"`
	CompressionScope         types.String `tfsdk:"compression_scope"`
	OnboardingComplete       types.Bool   `tfsdk:"onboarding_complete"`
	ConvertToolResultsToToon types.Bool   `tfsdk:"convert_tool_results_to_toon"`

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
	ShowTwoFactor           types.Bool   `tfsdk:"show_two_factor"`
	SlimChatErrorUI         types.Bool   `tfsdk:"slim_chat_error_ui"`

	// Security settings
	GlobalToolPolicy     types.String `tfsdk:"global_tool_policy"`
	AllowChatFileUploads types.Bool   `tfsdk:"allow_chat_file_uploads"`

	// Agent settings
	DefaultLlmModel    types.String `tfsdk:"default_llm_model"`
	DefaultLlmProvider types.String `tfsdk:"default_llm_provider"`
	DefaultLlmApiKeyId types.String `tfsdk:"default_llm_api_key_id"`
	DefaultAgentId     types.String `tfsdk:"default_agent_id"`

	// MCP settings
	McpOauthAccessTokenLifetimeSeconds types.Int64 `tfsdk:"mcp_oauth_access_token_lifetime_seconds"`

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
		MarkdownDescription: "Manages organization settings in Archestra. This is a singleton resource — only one instance can exist per organization.\n\n" +
			"**Lifecycle semantics depend on whether the field has a `Default:`.**\n\n" +
			"- Fields with a documented default (`font`, `color_theme`, `compression_scope`, `convert_tool_results_to_toon`): omitting the field from your `.tf` resets it to the default on the next apply. To preserve the current backend value, set the field explicitly.\n" +
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
				Default:             stringdefault.StaticString(string(client.Lato)),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.Inter),
						string(client.JetbrainsMono),
						string(client.Lato),
						string(client.OpenSans),
						string(client.Roboto),
						string(client.SourceSansPro),
					),
				},
			},
			"color_theme": schema.StringAttribute{
				MarkdownDescription: "Color theme for the organization UI",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(client.CosmicNight)),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.AmberMinimal),
						string(client.BoxyMinimalistic),
						string(client.Bubblegum),
						string(client.Caffeine),
						string(client.Catppuccin),
						string(client.Claude),
						string(client.CleanSlate),
						string(client.CosmicNight),
						string(client.Doom64),
						string(client.DraculaDark),
						string(client.GruvboxDark),
						string(client.MochaMousse),
						string(client.ModernMinimal),
						string(client.Mono),
						string(client.MonokaiDark),
						string(client.MoonlightDark),
						string(client.Nature),
						string(client.NeoBrutalism),
						string(client.SolarizedDark),
						string(client.SunsetHorizon),
						string(client.Tangerine),
						string(client.Twitter),
						string(client.Vercel),
						string(client.VintagePaper),
					),
				},
			},
			"logo": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded logo image for the organization",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"limit_cleanup_interval": schema.StringAttribute{
				MarkdownDescription: "Interval for cleaning up usage limits. Valid values: 1h, 12h, 24h, 1w, 1m. Set to null to disable.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.N1h),
						string(client.N12h),
						string(client.N24h),
						string(client.N1w),
						string(client.N1m),
					),
				},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"compression_scope": schema.StringAttribute{
				MarkdownDescription: "Scope for tool results compression",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(client.Organization)),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.Organization),
						string(client.Team),
					),
				},
			},
			"onboarding_complete": schema.BoolAttribute{
				MarkdownDescription: "Whether organization onboarding is complete. This is a one-way flag — once set to `true`, it cannot be reverted to `false`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"convert_tool_results_to_toon": schema.BoolAttribute{
				MarkdownDescription: "Whether to convert tool results to TOON format for compression",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
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
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"animate_chat_placeholders": schema.BoolAttribute{
				MarkdownDescription: "Whether to animate chat placeholders in the UI",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"show_two_factor": schema.BoolAttribute{
				MarkdownDescription: "Whether to show two-factor authentication options",
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
			"global_tool_policy": schema.StringAttribute{
				MarkdownDescription: "Global tool invocation policy. Valid values: permissive, restrictive.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("permissive", "restrictive"),
				},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"allow_chat_file_uploads": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow file uploads in chat",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},

			// Agent settings
			"default_llm_model": schema.StringAttribute{
				MarkdownDescription: "Default LLM model for the organization",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"default_llm_provider": schema.StringAttribute{
				MarkdownDescription: "Default LLM provider for the organization. One of the providers supported by `archestra_llm_provider_api_key.llm_provider`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.CreateLlmProviderApiKeyJSONBodyProviderAnthropic),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderAzure),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderBedrock),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderCerebras),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderCohere),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderDeepseek),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderGemini),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderGroq),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderMinimax),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderMistral),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderOllama),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderOpenai),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderOpenrouter),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderPerplexity),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderVllm),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderXai),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderZhipuai),
					),
				},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
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
			"mcp_oauth_access_token_lifetime_seconds": schema.Int64Attribute{
				MarkdownDescription: "Lifetime in seconds for MCP OAuth access tokens. Must be at least 1 second.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},

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

	prior := tftypes.NewValue(req.Plan.Raw.Type(), nil)
	r.applySettings(ctx, req.Plan.Raw, prior, &resp.Diagnostics)
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
		{"appearance", orgSettingsAppearanceSpec, func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateAppearanceSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"llm", orgSettingsLlmSpec, func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateLlmSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"security", orgSettingsSecuritySpec, func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateSecuritySettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"agent", orgSettingsAgentSpec, func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateAgentSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"mcp", orgSettingsMcpSpec, func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateMcpSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		}},
		{"knowledge", orgSettingsKnowledgeSpec, func(body io.Reader) (int, []byte, error) {
			resp, err := r.client.UpdateKnowledgeSettingsWithBodyWithResponse(ctx, "application/json", body)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
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
	apiResp, err := r.client.GetOrganizationWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read organization settings, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Font = types.StringValue(string(apiResp.JSON200.CustomFont))
	data.ColorTheme = types.StringValue(string(apiResp.JSON200.Theme))
	data.CompressionScope = types.StringValue(string(apiResp.JSON200.CompressionScope))
	data.OnboardingComplete = types.BoolValue(apiResp.JSON200.OnboardingComplete)
	data.ConvertToolResultsToToon = types.BoolValue(apiResp.JSON200.ConvertToolResultsToToon)

	if apiResp.JSON200.Logo != nil {
		data.Logo = types.StringValue(*apiResp.JSON200.Logo)
	} else {
		data.Logo = types.StringNull()
	}

	if apiResp.JSON200.LimitCleanupInterval != nil {
		data.LimitCleanupInterval = types.StringValue(string(*apiResp.JSON200.LimitCleanupInterval))
	} else {
		data.LimitCleanupInterval = types.StringNull()
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
	data.ShowTwoFactor = types.BoolValue(apiResp.JSON200.ShowTwoFactor)
	data.SlimChatErrorUI = types.BoolValue(apiResp.JSON200.SlimChatErrorUi)

	// Security settings
	data.GlobalToolPolicy = types.StringValue(string(apiResp.JSON200.GlobalToolPolicy))
	data.AllowChatFileUploads = types.BoolValue(apiResp.JSON200.AllowChatFileUploads)

	// Agent settings
	if apiResp.JSON200.DefaultLlmModel != nil {
		data.DefaultLlmModel = types.StringValue(*apiResp.JSON200.DefaultLlmModel)
	} else {
		data.DefaultLlmModel = types.StringNull()
	}
	if apiResp.JSON200.DefaultLlmProvider != nil {
		data.DefaultLlmProvider = types.StringValue(string(*apiResp.JSON200.DefaultLlmProvider))
	} else {
		data.DefaultLlmProvider = types.StringNull()
	}
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
	data.McpOauthAccessTokenLifetimeSeconds = types.Int64Value(int64(apiResp.JSON200.McpOauthAccessTokenLifetimeSeconds))

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
