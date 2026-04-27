package provider

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
