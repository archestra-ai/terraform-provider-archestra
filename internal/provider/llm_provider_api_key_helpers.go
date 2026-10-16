package provider

import (
	"context"
	"encoding/json"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// llmProviderApiKeyAttrSpec covers the LLM provider API key body for both
// CreateLlmProviderApiKey and UpdateLlmProviderApiKey. `llm_provider` is
// RequiresReplace — Update never sees it change, but it ships in the Create
// body as `provider`.
var llmProviderApiKeyAttrSpec = []AttrSpec{
	{TFName: "aws_access_key_id", JSONName: "awsAccessKeyId", Kind: Scalar, Sensitive: true, OmitOnNull: true},
	{TFName: "aws_secret_access_key", JSONName: "awsSecretAccessKey", Kind: Scalar, Sensitive: true, OmitOnNull: true},
	{TFName: "aws_session_token", JSONName: "awsSessionToken", Kind: Scalar, Sensitive: true, OmitOnNull: true},
	{TFName: "shared", JSONName: "shared", Kind: Scalar, OmitOnNull: true},
	{TFName: "extra_headers", JSONName: "extraHeaders", Kind: Map, Sensitive: true},
	{TFName: "inference_base_url", JSONName: "inferenceBaseUrl", Kind: Scalar},
	{TFName: "labels", JSONName: "labels", Kind: Set, OmitOnNull: true, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "llm_provider", JSONName: "provider", Kind: Scalar},
	{TFName: "api_key", JSONName: "apiKey", Kind: Scalar, Sensitive: true},
	{TFName: "is_primary", JSONName: "isPrimary", Kind: Scalar},
	{TFName: "base_url", JSONName: "baseUrl", Kind: Scalar},
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
		"isSystem",
	}
}

func (r *LLMProviderApiKeyResource) refreshKey(ctx context.Context, data *LLMProviderApiKeyResourceModel, diags *diag.Diagnostics) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Key ID", "The API returned an invalid key identifier.")
		return
	}
	result, err := r.client.GetLlmProviderApiKeyWithResponse(ctx, id)
	if err != nil {
		diags.AddError("Key Refresh Failed", err.Error())
		return
	}
	if result.JSON200 == nil {
		diags.AddError("Key Refresh Failed", "The saved key could not be read after writing.")
		return
	}
	r.flattenKey(ctx, data, result, diags)
}

func (r *LLMProviderApiKeyResource) flattenKey(ctx context.Context, data *LLMProviderApiKeyResourceModel, response *client.GetLlmProviderApiKeyResponse, diags *diag.Diagnostics) {
	wire := response.JSON200
	data.Name = types.StringValue(wire.Name)
	data.LLMProvider = types.StringValue(string(wire.Provider))
	data.IsPrimary = types.BoolValue(wire.IsPrimary)
	data.IsAgentKey = types.BoolValue(wire.IsAgentKey != nil && *wire.IsAgentKey)
	data.Scope = types.StringValue(string(wire.Scope))
	data.Shared = types.BoolValue(string(wire.Scope) != "personal")
	data.BaseUrl = types.StringPointerValue(wire.BaseUrl)
	data.TeamID = types.StringPointerValue(wire.TeamId)
	data.InferenceBaseURL = types.StringPointerValue(wire.InferenceBaseUrl)
	if wire.ExtraHeaders == nil {
		data.ExtraHeaders = types.MapNull(types.StringType)
	} else {
		value, ds := types.MapValueFrom(ctx, types.StringType, *wire.ExtraHeaders)
		diags.Append(ds...)
		data.ExtraHeaders = value
	}
	if wire.Labels != nil {
		data.Labels = flattenAgentLabels(data.Labels, *wire.Labels)
	} else {
		data.Labels = flattenAgentLabels(data.Labels, nil)
	}
	data.CreatedBy = types.StringNull()
	if wire.CreatedBy != nil {
		encoded, err := json.Marshal(wire.CreatedBy)
		if err != nil {
			diags.AddError("Invalid Creator Metadata", "Unable to encode the key creator identity.")
			return
		}
		data.CreatedBy = types.StringValue(string(encoded))
	}
	data.CreatedByServiceAccountID = types.StringNull()
	if wire.CreatedByServiceAccountId != nil {
		data.CreatedByServiceAccountID = types.StringValue(wire.CreatedByServiceAccountId.String())
	}
	data.RequiresReauthentication = types.BoolValue(wire.RequiresReauthentication != nil && *wire.RequiresReauthentication)
	data.SubscriptionKind = types.StringNull()
	if wire.SubscriptionKind != nil {
		data.SubscriptionKind = types.StringValue(string(*wire.SubscriptionKind))
	}
	// The API does not return inline secrets or always return vault references.
	if wire.VaultSecretPath != nil {
		data.VaultSecretPath = types.StringValue(*wire.VaultSecretPath)
	}
	if wire.VaultSecretKey != nil {
		data.VaultSecretKey = types.StringValue(*wire.VaultSecretKey)
	}
}
