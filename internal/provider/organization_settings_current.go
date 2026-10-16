package provider

import (
	"bytes"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Current organization settings are split by their actual API write domain.
func currentOrganizationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"online_mcp_catalog_enabled":                   schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether the online MCP catalog is available."},
		"online_skill_catalog_enabled":                 schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether the online skill catalog is available."},
		"skill_marketplace_anonymous_access":           schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether anonymous users can access the skill marketplace."},
		"allow_tool_auto_assignment":                   schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether new tools can be assigned automatically."},
		"ocr_chat_api_key_id":                          schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Provider key used for OCR requests."},
		"ocr_model":                                    schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Model used for OCR requests."},
		"kb_bm25_k1":                                   schema.Float64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Float64{float64planmodifier.UseStateForUnknown()}, MarkdownDescription: "BM25 term-frequency saturation coefficient."},
		"kb_bm25_b":                                    schema.Float64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Float64{float64planmodifier.UseStateForUnknown()}, MarkdownDescription: "BM25 document-length normalization coefficient."},
		"kb_contextual_retrieval_mode":                 schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Contextual retrieval mode for knowledge chunks."},
		"default_model_id":                             schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Database ID of the default discovered LLM model."},
		"default_member_role":                          schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Role assigned to new organization members."},
		"onboarding_wizard":                            schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for onboarding wizard."},
		"icon_logo_dark":                               schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Icon logo displayed in dark mode."},
		"require_two_factor":                           schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether organization members must use two-factor authentication."},
		"session_max_age_seconds":                      schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}, MarkdownDescription: "Maximum authentication session age in seconds."},
		"oauth_access_token_lifetime_seconds":          schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}, MarkdownDescription: "Lifetime of issued OAuth access tokens in seconds."},
		"connection_default_mcp_gateway_id":            schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Default gateway offered to connecting clients."},
		"connection_default_client_id":                 schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Default client shown by connection setup."},
		"connection_shown_client_ids":                  schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for connection shown client ids."},
		"connection_client_order":                      schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for connection client order."},
		"connection_base_urls":                         connectionBaseURLSchema(),
		"connection_client_future_providers":           schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for connection client future providers."},
		"connection_client_provider_keys":              schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for connection client provider keys."},
		"connection_skills_enabled":                    schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether connection setup offers skills."},
		"connection_llm_proxy_enabled":                 schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether connection setup offers the LLM proxy."},
		"connection_plugins_enabled":                   schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether connection setup offers plugins."},
		"connection_runtime_handoff_enabled":           schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether clients can hand work to an agent runtime."},
		"connection_runtime_handoff_instructions":      schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Instructions shown during runtime handoff."},
		"model_provider_overrides":                     schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for model provider overrides."},
		"messaging_channel_overrides":                  schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for messaging channel overrides."},
		"knowledge_connector_overrides":                schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for knowledge connector overrides."},
		"popular_agent_overrides":                      schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for popular agent overrides."},
		"default_environment_name":                     schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Name of the default deployment environment."},
		"default_environment_namespace":                schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Kubernetes namespace of the default deployment environment."},
		"default_environment_description":              schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Description of the default deployment environment."},
		"default_network_policy":                       schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for default network policy."},
		"default_environment_validation_regex":         schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Validation expression for default-environment deployment names."},
		"default_environment_trusted_image_registries": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON configuration for default environment trusted image registries."},
		"mcp_idle_hibernation_enabled":                 schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether idle MCP deployments can hibernate."},
		"apps_hackathon_recorder_enabled":              schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether the app recorder is enabled."},
		"new_apps_disabled_by_default":                 schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether new apps start disabled."},
		"new_apps_locked_by_default":                   schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether new apps start locked."},
	}
}
func currentOrganizationSpecs(domain string) []AttrSpec {
	switch domain {
	case "mcp-settings":
		return []AttrSpec{
			{TFName: "online_mcp_catalog_enabled", JSONName: "onlineMcpCatalogEnabled", Kind: Scalar},
			{TFName: "mcp_idle_hibernation_enabled", JSONName: "mcpIdleHibernationEnabled", Kind: Scalar},
		}
	case "skills-settings":
		return []AttrSpec{
			{TFName: "online_skill_catalog_enabled", JSONName: "onlineSkillCatalogEnabled", Kind: Scalar},
			{TFName: "skill_marketplace_anonymous_access", JSONName: "skillMarketplaceAnonymousAccess", Kind: Scalar},
		}
	case "security-settings":
		return []AttrSpec{
			{TFName: "allow_tool_auto_assignment", JSONName: "allowToolAutoAssignment", Kind: Scalar},
			{TFName: "apps_hackathon_recorder_enabled", JSONName: "appsHackathonRecorderEnabled", Kind: Scalar},
			{TFName: "new_apps_disabled_by_default", JSONName: "newAppsDisabledByDefault", Kind: Scalar},
			{TFName: "new_apps_locked_by_default", JSONName: "newAppsLockedByDefault", Kind: Scalar},
		}
	case "knowledge-settings":
		return []AttrSpec{
			{TFName: "ocr_chat_api_key_id", JSONName: "ocrChatApiKeyId", Kind: Scalar},
			{TFName: "ocr_model", JSONName: "ocrModel", Kind: Scalar},
			{TFName: "kb_bm25_k1", JSONName: "kbBm25K1", Kind: Scalar},
			{TFName: "kb_bm25_b", JSONName: "kbBm25B", Kind: Scalar},
			{TFName: "kb_contextual_retrieval_mode", JSONName: "kbContextualRetrievalMode", Kind: Scalar},
		}
	case "agent-settings":
		return []AttrSpec{
			{TFName: "default_model_id", JSONName: "defaultModelId", Kind: Scalar},
		}
	case "auth-settings":
		return []AttrSpec{
			{TFName: "default_member_role", JSONName: "defaultMemberRole", Kind: Scalar},
			{TFName: "require_two_factor", JSONName: "requireTwoFactor", Kind: Scalar},
			{TFName: "session_max_age_seconds", JSONName: "sessionMaxAgeSeconds", Kind: Scalar},
			{TFName: "oauth_access_token_lifetime_seconds", JSONName: "oauthAccessTokenLifetimeSeconds", Kind: Scalar},
		}
	case "appearance-settings":
		return []AttrSpec{
			{TFName: "onboarding_wizard", JSONName: "onboardingWizard", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "icon_logo_dark", JSONName: "iconLogoDark", Kind: Scalar},
		}
	case "connection-settings":
		return []AttrSpec{
			{TFName: "connection_default_mcp_gateway_id", JSONName: "connectionDefaultMcpGatewayId", Kind: Scalar},
			{TFName: "connection_default_client_id", JSONName: "connectionDefaultClientId", Kind: Scalar},
			{TFName: "connection_shown_client_ids", JSONName: "connectionShownClientIds", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "connection_client_order", JSONName: "connectionClientOrder", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "connection_base_urls", JSONName: "connectionBaseUrls", Kind: List, Children: connectionBaseURLSpecs},
			{TFName: "connection_client_future_providers", JSONName: "connectionClientFutureProviders", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "connection_client_provider_keys", JSONName: "connectionClientProviderKeys", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "connection_skills_enabled", JSONName: "connectionSkillsEnabled", Kind: Scalar},
			{TFName: "connection_llm_proxy_enabled", JSONName: "connectionLlmProxyEnabled", Kind: Scalar},
			{TFName: "connection_plugins_enabled", JSONName: "connectionPluginsEnabled", Kind: Scalar},
			{TFName: "connection_runtime_handoff_enabled", JSONName: "connectionRuntimeHandoffEnabled", Kind: Scalar},
			{TFName: "connection_runtime_handoff_instructions", JSONName: "connectionRuntimeHandoffInstructions", Kind: Scalar},
		}
	case "integration-settings":
		return []AttrSpec{
			{TFName: "model_provider_overrides", JSONName: "modelProviderOverrides", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "messaging_channel_overrides", JSONName: "messagingChannelOverrides", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "knowledge_connector_overrides", JSONName: "knowledgeConnectorOverrides", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "popular_agent_overrides", JSONName: "popularAgentOverrides", Kind: Scalar, Encoder: organizationJSONSetting},
		}
	case "default-environment":
		return []AttrSpec{
			{TFName: "default_environment_name", JSONName: "name", Kind: Scalar},
			{TFName: "default_environment_namespace", JSONName: "namespace", Kind: Scalar},
			{TFName: "default_environment_description", JSONName: "description", Kind: Scalar},
			{TFName: "default_network_policy", JSONName: "networkPolicy", Kind: Scalar, Encoder: organizationJSONSetting},
			{TFName: "default_environment_validation_regex", JSONName: "validationRegex", Kind: Scalar},
			{TFName: "default_environment_trusted_image_registries", JSONName: "trustedImageRegistries", Kind: Scalar, Encoder: organizationJSONSetting},
		}
	}
	return nil
}
func allCurrentOrganizationSpecs() []AttrSpec {
	var result []AttrSpec
	for _, domain := range []string{"mcp-settings", "skills-settings", "security-settings", "knowledge-settings", "agent-settings", "auth-settings", "appearance-settings", "connection-settings", "integration-settings", "default-environment"} {
		result = append(result, currentOrganizationSpecs(domain)...)
	}
	return result
}
func organizationJSONSetting(value any) any {
	if text, ok := value.(string); ok {
		return json.RawMessage(text)
	}
	return value
}
func flattenCurrentOrganizationSettings(data *OrganizationSettingsResourceModel, body []byte, diags *diag.Diagnostics) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(body, &values); err != nil {
		diags.AddError("Invalid Organization Response", "Unable to decode organization settings.")
		return
	}
	{
		var value *bool
		if raw := values["onlineMcpCatalogEnabled"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for onlineMcpCatalogEnabled.")
				return
			}
		}
		data.OnlineMcpCatalogEnabled = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["onlineSkillCatalogEnabled"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for onlineSkillCatalogEnabled.")
				return
			}
		}
		data.OnlineSkillCatalogEnabled = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["skillMarketplaceAnonymousAccess"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for skillMarketplaceAnonymousAccess.")
				return
			}
		}
		data.SkillMarketplaceAnonymousAccess = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["allowToolAutoAssignment"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for allowToolAutoAssignment.")
				return
			}
		}
		data.AllowToolAutoAssignment = types.BoolPointerValue(value)
	}
	{
		var value *string
		if raw := values["ocrChatApiKeyId"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for ocrChatApiKeyId.")
				return
			}
		}
		data.OcrChatApiKeyId = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["ocrModel"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for ocrModel.")
				return
			}
		}
		data.OcrModel = types.StringPointerValue(value)
	}
	{
		var value *float64
		if raw := values["kbBm25K1"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for kbBm25K1.")
				return
			}
		}
		data.KbBm25K1 = types.Float64PointerValue(value)
	}
	{
		var value *float64
		if raw := values["kbBm25B"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for kbBm25B.")
				return
			}
		}
		data.KbBm25B = types.Float64PointerValue(value)
	}
	{
		var value *string
		if raw := values["kbContextualRetrievalMode"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for kbContextualRetrievalMode.")
				return
			}
		}
		data.KbContextualRetrievalMode = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["defaultModelId"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for defaultModelId.")
				return
			}
		}
		data.DefaultModelId = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["defaultMemberRole"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for defaultMemberRole.")
				return
			}
		}
		data.DefaultMemberRole = types.StringPointerValue(value)
	}
	if raw := values["onboardingWizard"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.OnboardingWizard = value
	} else if data.OnboardingWizard.ValueString() != "null" {
		data.OnboardingWizard = jsontypes.NewNormalizedNull()
	}
	{
		var value *string
		if raw := values["iconLogoDark"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for iconLogoDark.")
				return
			}
		}
		data.IconLogoDark = types.StringPointerValue(value)
	}
	{
		var value *bool
		if raw := values["requireTwoFactor"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for requireTwoFactor.")
				return
			}
		}
		data.RequireTwoFactor = types.BoolPointerValue(value)
	}
	{
		var value *int64
		if raw := values["sessionMaxAgeSeconds"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for sessionMaxAgeSeconds.")
				return
			}
		}
		data.SessionMaxAgeSeconds = types.Int64PointerValue(value)
	}
	{
		var value *int64
		if raw := values["oauthAccessTokenLifetimeSeconds"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for oauthAccessTokenLifetimeSeconds.")
				return
			}
		}
		data.OauthAccessTokenLifetimeSeconds = types.Int64PointerValue(value)
	}
	{
		var value *string
		if raw := values["connectionDefaultMcpGatewayId"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for connectionDefaultMcpGatewayId.")
				return
			}
		}
		data.ConnectionDefaultMcpGatewayId = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["connectionDefaultClientId"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for connectionDefaultClientId.")
				return
			}
		}
		data.ConnectionDefaultClientId = types.StringPointerValue(value)
	}
	if raw := values["connectionShownClientIds"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.ConnectionShownClientIds = value
	} else if data.ConnectionShownClientIds.ValueString() != "null" {
		data.ConnectionShownClientIds = jsontypes.NewNormalizedNull()
	}
	if raw := values["connectionClientOrder"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.ConnectionClientOrder = value
	} else if data.ConnectionClientOrder.ValueString() != "null" {
		data.ConnectionClientOrder = jsontypes.NewNormalizedNull()
	}
	flattenConnectionBaseURLs(&data.ConnectionBaseUrls, values["connectionBaseUrls"], diags)
	if diags.HasError() {
		return
	}
	if raw := values["connectionClientFutureProviders"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.ConnectionClientFutureProviders = value
	} else if data.ConnectionClientFutureProviders.ValueString() != "null" {
		data.ConnectionClientFutureProviders = jsontypes.NewNormalizedNull()
	}
	if raw := values["connectionClientProviderKeys"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.ConnectionClientProviderKeys = value
	} else if data.ConnectionClientProviderKeys.ValueString() != "null" {
		data.ConnectionClientProviderKeys = jsontypes.NewNormalizedNull()
	}
	{
		var value *bool
		if raw := values["connectionSkillsEnabled"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for connectionSkillsEnabled.")
				return
			}
		}
		data.ConnectionSkillsEnabled = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["connectionLlmProxyEnabled"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for connectionLlmProxyEnabled.")
				return
			}
		}
		data.ConnectionLlmProxyEnabled = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["connectionPluginsEnabled"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for connectionPluginsEnabled.")
				return
			}
		}
		data.ConnectionPluginsEnabled = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["connectionRuntimeHandoffEnabled"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for connectionRuntimeHandoffEnabled.")
				return
			}
		}
		data.ConnectionRuntimeHandoffEnabled = types.BoolPointerValue(value)
	}
	{
		var value *string
		if raw := values["connectionRuntimeHandoffInstructions"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for connectionRuntimeHandoffInstructions.")
				return
			}
		}
		data.ConnectionRuntimeHandoffInstructions = types.StringPointerValue(value)
	}
	if raw := values["modelProviderOverrides"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.ModelProviderOverrides = value
	} else if data.ModelProviderOverrides.ValueString() != "null" {
		data.ModelProviderOverrides = jsontypes.NewNormalizedNull()
	}
	if raw := values["messagingChannelOverrides"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.MessagingChannelOverrides = value
	} else if data.MessagingChannelOverrides.ValueString() != "null" {
		data.MessagingChannelOverrides = jsontypes.NewNormalizedNull()
	}
	if raw := values["knowledgeConnectorOverrides"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.KnowledgeConnectorOverrides = value
	} else if data.KnowledgeConnectorOverrides.ValueString() != "null" {
		data.KnowledgeConnectorOverrides = jsontypes.NewNormalizedNull()
	}
	if raw := values["popularAgentOverrides"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.PopularAgentOverrides = value
	} else if data.PopularAgentOverrides.ValueString() != "null" {
		data.PopularAgentOverrides = jsontypes.NewNormalizedNull()
	}
	{
		var value *string
		if raw := values["defaultEnvironmentName"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for defaultEnvironmentName.")
				return
			}
		}
		data.DefaultEnvironmentName = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["defaultEnvironmentNamespace"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for defaultEnvironmentNamespace.")
				return
			}
		}
		data.DefaultEnvironmentNamespace = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["defaultEnvironmentDescription"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for defaultEnvironmentDescription.")
				return
			}
		}
		data.DefaultEnvironmentDescription = types.StringPointerValue(value)
	}
	if raw := values["defaultNetworkPolicy"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.DefaultNetworkPolicy = value
	} else if data.DefaultNetworkPolicy.ValueString() != "null" {
		data.DefaultNetworkPolicy = jsontypes.NewNormalizedNull()
	}
	{
		var value *string
		if raw := values["defaultEnvironmentValidationRegex"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for defaultEnvironmentValidationRegex.")
				return
			}
		}
		data.DefaultEnvironmentValidationRegex = types.StringPointerValue(value)
	}
	if raw := values["defaultEnvironmentTrustedImageRegistries"]; len(raw) > 0 && string(raw) != "null" {
		value, err := normalizedOrganizationJSON(raw)
		if err != nil {
			diags.AddError("Invalid Organization Response", "Unable to normalize JSON settings.")
			return
		}
		data.DefaultEnvironmentTrustedImageRegistries = value
	} else if data.DefaultEnvironmentTrustedImageRegistries.ValueString() != "null" {
		data.DefaultEnvironmentTrustedImageRegistries = jsontypes.NewNormalizedNull()
	}
	{
		var value *bool
		if raw := values["mcpIdleHibernationEnabled"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for mcpIdleHibernationEnabled.")
				return
			}
		}
		data.McpIdleHibernationEnabled = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["appsHackathonRecorderEnabled"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for appsHackathonRecorderEnabled.")
				return
			}
		}
		data.AppsHackathonRecorderEnabled = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["newAppsDisabledByDefault"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for newAppsDisabledByDefault.")
				return
			}
		}
		data.NewAppsDisabledByDefault = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["newAppsLockedByDefault"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Organization Response", "Unexpected type for newAppsLockedByDefault.")
				return
			}
		}
		data.NewAppsLockedByDefault = types.BoolPointerValue(value)
	}
}

func normalizedOrganizationJSON(raw json.RawMessage) (jsontypes.Normalized, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	return jsontypes.NewNormalizedValue(string(encoded)), nil
}
