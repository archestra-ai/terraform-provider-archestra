package provider

// Each backend org-settings endpoint owns a disjoint slice of fields. Splitting
// the AttrSpec by endpoint lets us run MergePatch per-endpoint over the same
// plan/prior tftypes.Value: an endpoint whose patch is empty skips the API
// call entirely, and a stale field can't smear across endpoints.
//
// The split mirrors platform/backend/src/routes/organization.ts. If a new field
// lands on a new endpoint, add it here and to the schema; AttrSpecs() (the
// drift-check union) catches a missing entry.

var orgSettingsAppearanceSpec = []AttrSpec{
	{TFName: "font", JSONName: "customFont", Kind: Scalar},
	{TFName: "color_theme", JSONName: "theme", Kind: Scalar},
	{TFName: "logo", JSONName: "logo", Kind: Scalar},
	{TFName: "logo_dark", JSONName: "logoDark", Kind: Scalar},
	{TFName: "favicon", JSONName: "favicon", Kind: Scalar},
	{TFName: "icon_logo", JSONName: "iconLogo", Kind: Scalar},
	{TFName: "app_name", JSONName: "appName", Kind: Scalar},
	{TFName: "footer_text", JSONName: "footerText", Kind: Scalar},
	{TFName: "og_description", JSONName: "ogDescription", Kind: Scalar},
	{TFName: "chat_error_support_message", JSONName: "chatErrorSupportMessage", Kind: Scalar},
	{TFName: "chat_placeholders", JSONName: "chatPlaceholders", Kind: List},
	{TFName: "chat_links", JSONName: "chatLinks", Kind: List, Children: []AttrSpec{
		{TFName: "label", JSONName: "label", Kind: Scalar},
		{TFName: "url", JSONName: "url", Kind: Scalar},
	}},
	{TFName: "animate_chat_placeholders", JSONName: "animateChatPlaceholders", Kind: Scalar},
	{TFName: "show_two_factor", JSONName: "showTwoFactor", Kind: Scalar},
	{TFName: "slim_chat_error_ui", JSONName: "slimChatErrorUi", Kind: Scalar},
}

var orgSettingsLlmSpec = []AttrSpec{
	{TFName: "compression_scope", JSONName: "compressionScope", Kind: Scalar},
	{TFName: "convert_tool_results_to_toon", JSONName: "convertToolResultsToToon", Kind: Scalar},
	{TFName: "limit_cleanup_interval", JSONName: "limitCleanupInterval", Kind: Scalar},
}

var orgSettingsSecuritySpec = []AttrSpec{
	{TFName: "global_tool_policy", JSONName: "globalToolPolicy", Kind: Scalar},
	{TFName: "allow_chat_file_uploads", JSONName: "allowChatFileUploads", Kind: Scalar},
}

var orgSettingsAgentSpec = []AttrSpec{
	{TFName: "default_llm_model", JSONName: "defaultLlmModel", Kind: Scalar},
	{TFName: "default_llm_provider", JSONName: "defaultLlmProvider", Kind: Scalar},
	{TFName: "default_llm_api_key_id", JSONName: "defaultLlmApiKeyId", Kind: Scalar},
	{TFName: "default_agent_id", JSONName: "defaultAgentId", Kind: Scalar},
}

var orgSettingsMcpSpec = []AttrSpec{
	{TFName: "mcp_oauth_access_token_lifetime_seconds", JSONName: "mcpOauthAccessTokenLifetimeSeconds", Kind: Scalar},
}

var orgSettingsKnowledgeSpec = []AttrSpec{
	{TFName: "embedding_model", JSONName: "embeddingModel", Kind: Scalar},
	{TFName: "embedding_chat_api_key_id", JSONName: "embeddingChatApiKeyId", Kind: Scalar},
	{TFName: "reranker_model", JSONName: "rerankerModel", Kind: Scalar},
	{TFName: "reranker_chat_api_key_id", JSONName: "rerankerChatApiKeyId", Kind: Scalar},
}

// orgSettingsOnboardingSpec covers the one-shot CompleteOnboarding endpoint.
// Its single field is also boolean-only on the wire; the caller fires the
// endpoint only when the plan flips false→true (or null→true).
var orgSettingsOnboardingSpec = []AttrSpec{
	{TFName: "onboarding_complete", JSONName: "onboardingComplete", Kind: Scalar},
}

// AttrSpecs implements resourceWithAttrSpec — the drift-check sees the union
// across endpoints. `id` is computed-only and excluded by the lint helper.
func (r *OrganizationSettingsResource) AttrSpecs() []AttrSpec {
	specs := make([]AttrSpec, 0, 32)
	specs = append(specs, orgSettingsAppearanceSpec...)
	specs = append(specs, orgSettingsLlmSpec...)
	specs = append(specs, orgSettingsSecuritySpec...)
	specs = append(specs, orgSettingsAgentSpec...)
	specs = append(specs, orgSettingsMcpSpec...)
	specs = append(specs, orgSettingsKnowledgeSpec...)
	specs = append(specs, orgSettingsOnboardingSpec...)
	return specs
}
