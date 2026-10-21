package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// apiKeyAttrSpec covers the wire body for CreateApiKey. The backend
// has no Update endpoint, so every input field is RequiresReplace —
// the AttrSpec is still useful for the schema↔wire drift check and
// for `LogPatch` debug output.
//
// `key` is the one-shot Create-only token (AWS-access-key shape) —
// Synthetic + Sensitive so MergePatch ignores it and TestSpecDrift's
// sensitive symmetry check still passes.
var apiKeyAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "expires_in_seconds", JSONName: "expiresIn", Kind: Scalar},
	{TFName: "key", Kind: Synthetic, Sensitive: true},
}

func (r *ApiKeyResource) AttrSpecs() []AttrSpec {
	return apiKeyAttrSpec
}

func (r *ApiKeyResource) APIShape() any {
	return client.GetApiKeyResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - userId: implicit from the API key authenticating the request
//     (this resource creates keys for the caller). Surfacing it would
//     duplicate the caller identity already required by the provider.
//   - permissions, metadata: backend-managed access-control surfaces.
//     Neither the Create body nor the read response exposes a write
//     path through this provider — they're set via Better Auth's
//     internal flows.
func (r *ApiKeyResource) KnownIntentionallySkipped() []string {
	return []string{"userId", "permissions", "metadata"}
}
