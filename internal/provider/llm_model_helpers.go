package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// llmModelAttrSpec covers the LlmModel update body. The model is adopted by
// looking up `model_id` (a Synthetic field — not in the body, used only at
// Create-time to discover the UUID).
//
// Computed-only fields (llm_provider, description, context_length,
// price_per_million_*, is_custom_price, price_source) are excluded by
// the drift-check helper.
var llmModelAttrSpec = []AttrSpec{
	{TFName: "model_id", Kind: Synthetic},
	{TFName: "custom_price_per_million_input", JSONName: "customPricePerMillionInput", Kind: Scalar},
	{TFName: "custom_price_per_million_output", JSONName: "customPricePerMillionOutput", Kind: Scalar},
	{TFName: "ignored", JSONName: "ignored", Kind: Scalar},
	{TFName: "input_modalities", JSONName: "inputModalities", Kind: List},
	{TFName: "output_modalities", JSONName: "outputModalities", Kind: List},
}

func (r *LlmModelResource) AttrSpecs() []AttrSpec { return llmModelAttrSpec }

func (r *LlmModelResource) APIShape() any { return client.GetModelsWithApiKeysResponse{} }

// KnownIntentionallySkipped — wire fields not modeled on archestra_llm_model:
//   - provider: wire field renames to schema's `llm_provider` (Computed-only,
//     surfaced via the snake-case schema lookup once the rename is bridged
//     here).
//   - apiKeys: list of associated API keys; managed via the separate
//     archestra_llm_provider_api_key resource.
//   - createdAt/updatedAt/lastSyncedAt: audit timestamps.
//   - bestModelId/isBest/isFastest/discoveredViaLlmProxy/embeddingDimensions/
//     externalId/supportsToolCalling/promptPricePerToken/
//     completionPricePerToken: backend-derived metadata exposed in the UI
//     for sorting/filtering, not part of the manage-this-model surface.
//     Could be surfaced as Computed-only fields later if asked.
func (r *LlmModelResource) KnownIntentionallySkipped() []string {
	return []string{
		"provider", "apiKeys", "createdAt", "updatedAt", "lastSyncedAt",
		"bestModelId", "isBest", "isFastest", "discoveredViaLlmProxy",
		"embeddingDimensions", "externalId", "supportsToolCalling",
		"promptPricePerToken", "completionPricePerToken",
	}
}
