package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		MarkdownDescription: "Manages organization settings in Archestra. This is a singleton resource - only one instance can exist per organization. Note: Running `terraform destroy` will only remove this resource from Terraform state; the organization settings will remain unchanged on the server.",

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
			},
			"limit_cleanup_interval": schema.StringAttribute{
				MarkdownDescription: "Interval for cleaning up usage limits. Valid values: 1h, 12h, 24h, 1w, 1m. Set to null to disable.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.N1h),
						string(client.N12h),
						string(client.N24h),
						string(client.N1w),
						string(client.N1m),
					),
				},
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
			},
			"favicon": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded favicon image for the organization",
				Optional:            true,
			},
			"icon_logo": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded icon logo image for the organization",
				Optional:            true,
			},
			"app_name": schema.StringAttribute{
				MarkdownDescription: "Custom application name displayed in the UI",
				Optional:            true,
			},
			"footer_text": schema.StringAttribute{
				MarkdownDescription: "Custom footer text displayed in the UI",
				Optional:            true,
			},
			"og_description": schema.StringAttribute{
				MarkdownDescription: "OG meta description for the organization, max 500 characters",
				Optional:            true,
			},
			"chat_error_support_message": schema.StringAttribute{
				MarkdownDescription: "Custom error support message displayed in the chat UI",
				Optional:            true,
			},
			"chat_placeholders": schema.ListAttribute{
				MarkdownDescription: "Chat placeholder texts displayed in the chat input",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"chat_links": schema.ListNestedAttribute{
				MarkdownDescription: "Chat links displayed in the chat UI",
				Optional:            true,
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
			},
			"animate_chat_placeholders": schema.BoolAttribute{
				MarkdownDescription: "Whether to animate chat placeholders in the UI",
				Optional:            true,
				Computed:            true,
			},
			"show_two_factor": schema.BoolAttribute{
				MarkdownDescription: "Whether to show two-factor authentication options",
				Optional:            true,
				Computed:            true,
			},
			"slim_chat_error_ui": schema.BoolAttribute{
				MarkdownDescription: "When enabled, renders a compact error UI in chat views.",
				Optional:            true,
				Computed:            true,
			},

			// Security settings
			"global_tool_policy": schema.StringAttribute{
				MarkdownDescription: "Global tool invocation policy. Valid values: permissive, restrictive.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("permissive", "restrictive"),
				},
			},
			"allow_chat_file_uploads": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow file uploads in chat",
				Optional:            true,
				Computed:            true,
			},

			// Agent settings
			"default_llm_model": schema.StringAttribute{
				MarkdownDescription: "Default LLM model for the organization",
				Optional:            true,
			},
			"default_llm_provider": schema.StringAttribute{
				MarkdownDescription: "Default LLM provider for the organization",
				Optional:            true,
			},
			"default_llm_api_key_id": schema.StringAttribute{
				MarkdownDescription: "Default LLM API key ID for the organization",
				Optional:            true,
			},
			"default_agent_id": schema.StringAttribute{
				MarkdownDescription: "Default agent (profile) ID for the organization",
				Optional:            true,
			},

			// MCP settings
			"mcp_oauth_access_token_lifetime_seconds": schema.Int64Attribute{
				MarkdownDescription: "Lifetime in seconds for MCP OAuth access tokens",
				Optional:            true,
				Computed:            true,
			},

			// Knowledge settings
			"embedding_model": schema.StringAttribute{
				MarkdownDescription: "Embedding model for knowledge base. **Warning: locked after first configuration.** Changing requires dropping embedding config via the API first.",
				Optional:            true,
			},
			"embedding_chat_api_key_id": schema.StringAttribute{
				MarkdownDescription: "API key ID for the embedding model. **Warning: locked after first configuration.** Changing requires dropping embedding config via the API first.",
				Optional:            true,
			},
			"reranker_model": schema.StringAttribute{
				MarkdownDescription: "Reranker model for knowledge base",
				Optional:            true,
			},
			"reranker_chat_api_key_id": schema.StringAttribute{
				MarkdownDescription: "API key ID for the reranker model",
				Optional:            true,
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

	r.applySettings(ctx, &data, &resp.Diagnostics)
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

	r.applySettings(ctx, &data, &resp.Diagnostics)
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

func (r *OrganizationSettingsResource) applySettings(ctx context.Context, data *OrganizationSettingsResourceModel, diags *diag.Diagnostics) {
	// Update appearance settings (font, theme, logo)
	appearanceBody := client.UpdateAppearanceSettingsJSONRequestBody{}
	if !data.Font.IsNull() && !data.Font.IsUnknown() {
		font := client.UpdateAppearanceSettingsJSONBodyCustomFont(data.Font.ValueString())
		appearanceBody.CustomFont = &font
	}
	if !data.ColorTheme.IsNull() && !data.ColorTheme.IsUnknown() {
		theme := client.UpdateAppearanceSettingsJSONBodyTheme(data.ColorTheme.ValueString())
		appearanceBody.Theme = &theme
	}
	if !data.Logo.IsNull() && !data.Logo.IsUnknown() {
		logo := data.Logo.ValueString()
		appearanceBody.Logo = &logo
	}
	if !data.LogoDark.IsNull() && !data.LogoDark.IsUnknown() {
		logoDark := data.LogoDark.ValueString()
		appearanceBody.LogoDark = &logoDark
	}
	if !data.Favicon.IsNull() && !data.Favicon.IsUnknown() {
		favicon := data.Favicon.ValueString()
		appearanceBody.Favicon = &favicon
	}
	if !data.IconLogo.IsNull() && !data.IconLogo.IsUnknown() {
		iconLogo := data.IconLogo.ValueString()
		appearanceBody.IconLogo = &iconLogo
	}
	if !data.AppName.IsNull() && !data.AppName.IsUnknown() {
		appName := data.AppName.ValueString()
		appearanceBody.AppName = &appName
	}
	if !data.FooterText.IsNull() && !data.FooterText.IsUnknown() {
		footerText := data.FooterText.ValueString()
		appearanceBody.FooterText = &footerText
	}
	if !data.OgDescription.IsNull() && !data.OgDescription.IsUnknown() {
		ogDesc := data.OgDescription.ValueString()
		appearanceBody.OgDescription = &ogDesc
	}
	if !data.ChatErrorSupportMessage.IsNull() && !data.ChatErrorSupportMessage.IsUnknown() {
		msg := data.ChatErrorSupportMessage.ValueString()
		appearanceBody.ChatErrorSupportMessage = &msg
	}
	if !data.ChatPlaceholders.IsNull() && !data.ChatPlaceholders.IsUnknown() {
		var placeholders []string
		diags.Append(data.ChatPlaceholders.ElementsAs(ctx, &placeholders, false)...)
		if diags.HasError() {
			return
		}
		appearanceBody.ChatPlaceholders = &placeholders
	}
	if !data.ChatLinks.IsNull() && !data.ChatLinks.IsUnknown() {
		var chatLinkModels []ChatLinkModel
		diags.Append(data.ChatLinks.ElementsAs(ctx, &chatLinkModels, false)...)
		if diags.HasError() {
			return
		}
		chatLinks := make([]struct {
			Label string `json:"label"`
			Url   string `json:"url"`
		}, len(chatLinkModels))
		for i, cl := range chatLinkModels {
			chatLinks[i].Label = cl.Label.ValueString()
			chatLinks[i].Url = cl.URL.ValueString()
		}
		appearanceBody.ChatLinks = &chatLinks
	}
	if !data.AnimateChatPlaceholders.IsNull() && !data.AnimateChatPlaceholders.IsUnknown() {
		animate := data.AnimateChatPlaceholders.ValueBool()
		appearanceBody.AnimateChatPlaceholders = &animate
	}
	if !data.ShowTwoFactor.IsNull() && !data.ShowTwoFactor.IsUnknown() {
		show := data.ShowTwoFactor.ValueBool()
		appearanceBody.ShowTwoFactor = &show
	}
	if !data.SlimChatErrorUI.IsNull() && !data.SlimChatErrorUI.IsUnknown() {
		v := data.SlimChatErrorUI.ValueBool()
		appearanceBody.SlimChatErrorUi = &v
	}

	appearanceResp, err := r.client.UpdateAppearanceSettingsWithResponse(ctx, appearanceBody)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to update appearance settings, got error: %s", err))
		return
	}
	if appearanceResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK from UpdateAppearanceSettings, got status %d: %s", appearanceResp.StatusCode(), string(appearanceResp.Body)))
		return
	}

	// Update LLM settings (compression, convertToolResultsToToon, limitCleanupInterval)
	llmBody := client.UpdateLlmSettingsJSONRequestBody{}
	if !data.CompressionScope.IsNull() && !data.CompressionScope.IsUnknown() {
		scope := client.UpdateLlmSettingsJSONBodyCompressionScope(data.CompressionScope.ValueString())
		llmBody.CompressionScope = &scope
	}
	if !data.ConvertToolResultsToToon.IsNull() && !data.ConvertToolResultsToToon.IsUnknown() {
		convert := data.ConvertToolResultsToToon.ValueBool()
		llmBody.ConvertToolResultsToToon = &convert
	}
	if !data.LimitCleanupInterval.IsNull() && !data.LimitCleanupInterval.IsUnknown() {
		interval := client.UpdateLlmSettingsJSONBodyLimitCleanupInterval(data.LimitCleanupInterval.ValueString())
		llmBody.LimitCleanupInterval = &interval
	}

	llmResp, err := r.client.UpdateLlmSettingsWithResponse(ctx, llmBody)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to update LLM settings, got error: %s", err))
		return
	}
	if llmResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK from UpdateLlmSettings, got status %d: %s", llmResp.StatusCode(), string(llmResp.Body)))
		return
	}

	// Update security settings
	securityBody := client.UpdateSecuritySettingsJSONRequestBody{}
	if !data.GlobalToolPolicy.IsNull() && !data.GlobalToolPolicy.IsUnknown() {
		policy := client.UpdateSecuritySettingsJSONBodyGlobalToolPolicy(data.GlobalToolPolicy.ValueString())
		securityBody.GlobalToolPolicy = &policy
	}
	if !data.AllowChatFileUploads.IsNull() && !data.AllowChatFileUploads.IsUnknown() {
		allow := data.AllowChatFileUploads.ValueBool()
		securityBody.AllowChatFileUploads = &allow
	}

	securityResp, err := r.client.UpdateSecuritySettingsWithResponse(ctx, securityBody)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to update security settings, got error: %s", err))
		return
	}
	if securityResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK from UpdateSecuritySettings, got status %d: %s", securityResp.StatusCode(), string(securityResp.Body)))
		return
	}

	// Update agent settings
	agentBody := client.UpdateAgentSettingsJSONRequestBody{}
	if !data.DefaultLlmModel.IsNull() && !data.DefaultLlmModel.IsUnknown() {
		model := data.DefaultLlmModel.ValueString()
		agentBody.DefaultLlmModel = &model
	}
	if !data.DefaultLlmProvider.IsNull() && !data.DefaultLlmProvider.IsUnknown() {
		provider := client.UpdateAgentSettingsJSONBodyDefaultLlmProvider(data.DefaultLlmProvider.ValueString())
		agentBody.DefaultLlmProvider = &provider
	}
	if !data.DefaultLlmApiKeyId.IsNull() && !data.DefaultLlmApiKeyId.IsUnknown() {
		parsedID, parseErr := uuid.Parse(data.DefaultLlmApiKeyId.ValueString())
		if parseErr != nil {
			diags.AddError("Invalid UUID", fmt.Sprintf("Unable to parse default_llm_api_key_id: %s", parseErr))
			return
		}
		agentBody.DefaultLlmApiKeyId = &parsedID
	}
	if !data.DefaultAgentId.IsNull() && !data.DefaultAgentId.IsUnknown() {
		parsedID, parseErr := uuid.Parse(data.DefaultAgentId.ValueString())
		if parseErr != nil {
			diags.AddError("Invalid UUID", fmt.Sprintf("Unable to parse default_agent_id: %s", parseErr))
			return
		}
		agentBody.DefaultAgentId = &parsedID
	}

	agentResp, err := r.client.UpdateAgentSettingsWithResponse(ctx, agentBody)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to update agent settings, got error: %s", err))
		return
	}
	if agentResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK from UpdateAgentSettings, got status %d: %s", agentResp.StatusCode(), string(agentResp.Body)))
		return
	}

	// Update MCP settings
	mcpBody := client.UpdateMcpSettingsJSONRequestBody{}
	if !data.McpOauthAccessTokenLifetimeSeconds.IsNull() && !data.McpOauthAccessTokenLifetimeSeconds.IsUnknown() {
		lifetime := int(data.McpOauthAccessTokenLifetimeSeconds.ValueInt64())
		mcpBody.McpOauthAccessTokenLifetimeSeconds = &lifetime
	}

	mcpResp, err := r.client.UpdateMcpSettingsWithResponse(ctx, mcpBody)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to update MCP settings, got error: %s", err))
		return
	}
	if mcpResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK from UpdateMcpSettings, got status %d: %s", mcpResp.StatusCode(), string(mcpResp.Body)))
		return
	}

	// Update knowledge settings
	knowledgeBody := client.UpdateKnowledgeSettingsJSONRequestBody{}
	if !data.EmbeddingModel.IsNull() && !data.EmbeddingModel.IsUnknown() {
		model := data.EmbeddingModel.ValueString()
		knowledgeBody.EmbeddingModel = &model
	}
	if !data.EmbeddingChatApiKeyId.IsNull() && !data.EmbeddingChatApiKeyId.IsUnknown() {
		parsedID, parseErr := uuid.Parse(data.EmbeddingChatApiKeyId.ValueString())
		if parseErr != nil {
			diags.AddError("Invalid UUID", fmt.Sprintf("Unable to parse embedding_chat_api_key_id: %s", parseErr))
			return
		}
		knowledgeBody.EmbeddingChatApiKeyId = &parsedID
	}
	if !data.RerankerModel.IsNull() && !data.RerankerModel.IsUnknown() {
		model := data.RerankerModel.ValueString()
		knowledgeBody.RerankerModel = &model
	}
	if !data.RerankerChatApiKeyId.IsNull() && !data.RerankerChatApiKeyId.IsUnknown() {
		parsedID, parseErr := uuid.Parse(data.RerankerChatApiKeyId.ValueString())
		if parseErr != nil {
			diags.AddError("Invalid UUID", fmt.Sprintf("Unable to parse reranker_chat_api_key_id: %s", parseErr))
			return
		}
		knowledgeBody.RerankerChatApiKeyId = &parsedID
	}

	knowledgeResp, err := r.client.UpdateKnowledgeSettingsWithResponse(ctx, knowledgeBody)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to update knowledge settings, got error: %s", err))
		return
	}
	if knowledgeResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK from UpdateKnowledgeSettings, got status %d: %s", knowledgeResp.StatusCode(), string(knowledgeResp.Body)))
		return
	}

	// Complete onboarding if requested
	if !data.OnboardingComplete.IsNull() && !data.OnboardingComplete.IsUnknown() && data.OnboardingComplete.ValueBool() {
		onboardingBody := client.CompleteOnboardingJSONRequestBody{
			OnboardingComplete: true,
		}
		onboardingResp, err := r.client.CompleteOnboardingWithResponse(ctx, onboardingBody)
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to complete onboarding, got error: %s", err))
			return
		}
		if onboardingResp.JSON200 == nil {
			diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK from CompleteOnboarding, got status %d: %s", onboardingResp.StatusCode(), string(onboardingResp.Body)))
			return
		}
	}
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
}
