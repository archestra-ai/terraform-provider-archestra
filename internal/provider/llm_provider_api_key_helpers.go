package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// llmProviderApiKeyAttrSpec covers the LLM provider API key body for both
// CreateLlmProviderApiKey and UpdateLlmProviderApiKey. `llm_provider` is
// RequiresReplace — Update never sees it change, but it ships in the Create
// body as `provider`.
var llmProviderApiKeyAttrSpec = []AttrSpec{
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

func (r *LLMProviderApiKeyResource) AttrSpecs() []AttrSpec {
	return llmProviderApiKeyAttrSpec
}

func (r *LLMProviderApiKeyResource) APIShape() any {
	return client.GetLlmProviderApiKeyResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - createdAt/updatedAt: audit timestamps.
//   - organizationId/userId/userName/teamName: ownership metadata. The
//     team_id (already in the schema) covers team scoping; the rest is
//     backend display sugar.
//   - secretId/secretStorageType: BYOS/READONLY_VAULT internal bookkeeping
//     fields (which secret-manager backend stored the key); the user-facing
//     vault_secret_path/vault_secret_key already cover the BYOS path.
//   - bestModelId: cached "best model" pointer maintained by the backend's
//     model-discovery scheduler; not part of the user's create/update flow.
//   - isAgentKey/isSystem: server-side flags that mark special keys
//     created by built-in agents or system-level integrations; users
//     don't create these via Terraform.
func (r *LLMProviderApiKeyResource) KnownIntentionallySkipped() []string {
	return []string{
		"createdAt", "updatedAt", "organizationId", "userId", "userName",
		"teamName", "secretId", "secretStorageType", "bestModelId",
		"isAgentKey", "isSystem",
	}
}
