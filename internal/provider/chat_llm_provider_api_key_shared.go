package provider

// chatLlmProviderApiKeyAttrSpec covers the LLM provider API key body for both
// CreateLlmProviderApiKey and UpdateLlmProviderApiKey. `llm_provider` is
// RequiresReplace — Update never sees it change, but it ships in the Create
// body as `provider`.
var chatLlmProviderApiKeyAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "llm_provider", JSONName: "provider", Kind: Scalar},
	{TFName: "api_key", JSONName: "apiKey", Kind: Scalar, Sensitive: true},
	{TFName: "is_organization_default", JSONName: "isPrimary", Kind: Scalar},
	{TFName: "base_url", JSONName: "baseUrl", Kind: Scalar},
	{TFName: "scope", JSONName: "scope", Kind: Scalar},
	{TFName: "team_id", JSONName: "teamId", Kind: Scalar},
	{TFName: "vault_secret_path", JSONName: "vaultSecretPath", Kind: Scalar},
	{TFName: "vault_secret_key", JSONName: "vaultSecretKey", Kind: Scalar},
}

func (r *ChatLLMProviderApiKeyResource) AttrSpecs() []AttrSpec {
	return chatLlmProviderApiKeyAttrSpec
}
