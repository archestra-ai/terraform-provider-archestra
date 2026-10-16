package provider

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func currentModelAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"effective_input_modalities":           schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Current model input modalities, including provider-discovered values."},
		"effective_output_modalities":          schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Current model output modalities, including provider-discovered values."},
		"custom_price_per_million_cache_read":  schema.StringAttribute{Optional: true, MarkdownDescription: "Custom price per million cache read."},
		"custom_price_per_million_cache_write": schema.StringAttribute{Optional: true, MarkdownDescription: "Custom price per million cache write."},
		"cache_price_source":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Cache price source."},
		"cache_read_price_per_token":           schema.StringAttribute{Computed: true, MarkdownDescription: "Cache read price per token."},
		"cache_write_price_per_token":          schema.StringAttribute{Computed: true, MarkdownDescription: "Cache write price per token."},
		"price_per_million_cache_read":         schema.StringAttribute{Computed: true, MarkdownDescription: "Price per million cache read."},
		"price_per_million_cache_write":        schema.StringAttribute{Computed: true, MarkdownDescription: "Price per million cache write."},
		"custom_context_length":                schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, MarkdownDescription: "Custom context length in tokens."},
		"custom_output_length":                 schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, MarkdownDescription: "Custom output length in tokens."},
		"output_length":                        schema.Int64Attribute{Computed: true, MarkdownDescription: "Output length in tokens."},
		"embedding_client_image_capable":       schema.BoolAttribute{Computed: true, MarkdownDescription: "Embedding client image capable."},
		"is_free":                              schema.BoolAttribute{Computed: true, MarkdownDescription: "Is free."},
		"supports_reasoning_effort":            schema.BoolAttribute{Computed: true, MarkdownDescription: "Supports reasoning effort."},
		"configured_parameters":                schema.StringAttribute{Optional: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Configured parameters as JSON."},
		"default_parameters":                   schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Default parameters as JSON."},
		"teams":                                schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Teams as JSON."},
		"users":                                schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Users as JSON."},
		"effective_context_length":             schema.Float64Attribute{Computed: true, MarkdownDescription: "Context window after applying configured limits."},
		"supported_endpoints":                  schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Supported inference endpoints."},
		"labels":                               schema.SetNestedAttribute{Optional: true, MarkdownDescription: "Labels assigned to the model.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"key": schema.StringAttribute{Required: true}, "value": schema.StringAttribute{Required: true}}}},
	}
}
func flattenCurrentModelFields(ctx context.Context, data *LlmModelResourceModel, model any, diags *diag.Diagnostics) {
	encoded, err := json.Marshal(model)
	if err != nil {
		diags.AddError("Invalid Model Response", "Unable to encode model metadata.")
		return
	}
	var wire struct {
		InputModalities                 []string        `json:"inputModalities"`
		OutputModalities                []string        `json:"outputModalities"`
		CustomPricePerMillionCacheRead  *string         `json:"customPricePerMillionCacheRead"`
		CustomPricePerMillionCacheWrite *string         `json:"customPricePerMillionCacheWrite"`
		CachePriceSource                *string         `json:"cachePriceSource"`
		CacheReadPricePerToken          *string         `json:"cacheReadPricePerToken"`
		CacheWritePricePerToken         *string         `json:"cacheWritePricePerToken"`
		PricePerMillionCacheRead        *string         `json:"pricePerMillionCacheRead"`
		PricePerMillionCacheWrite       *string         `json:"pricePerMillionCacheWrite"`
		CustomContextLength             *int64          `json:"customContextLength"`
		CustomOutputLength              *int64          `json:"customOutputLength"`
		OutputLength                    *int64          `json:"outputLength"`
		EmbeddingClientImageCapable     *bool           `json:"embeddingClientImageCapable"`
		IsFree                          *bool           `json:"isFree"`
		SupportsReasoningEffort         *bool           `json:"supportsReasoningEffort"`
		ConfiguredParameters            json.RawMessage `json:"configuredParameters"`
		DefaultParameters               json.RawMessage `json:"defaultParameters"`
		Teams                           json.RawMessage `json:"teams"`
		Users                           json.RawMessage `json:"users"`
		EffectiveContextLength          *float64        `json:"effectiveContextLength"`
		SupportedEndpoints              []string        `json:"supportedEndpoints"`
		Labels                          []struct {
			Key     string              `json:"key"`
			KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
			Value   string              `json:"value"`
			ValueId *openapi_types.UUID `json:"valueId,omitempty"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		diags.AddError("Invalid Model Response", "Unable to decode model metadata.")
		return
	}
	data.CustomPricePerMillionCacheRead = types.StringPointerValue(wire.CustomPricePerMillionCacheRead)
	data.CustomPricePerMillionCacheWrite = types.StringPointerValue(wire.CustomPricePerMillionCacheWrite)
	data.CachePriceSource = types.StringPointerValue(wire.CachePriceSource)
	data.CacheReadPricePerToken = types.StringPointerValue(wire.CacheReadPricePerToken)
	data.CacheWritePricePerToken = types.StringPointerValue(wire.CacheWritePricePerToken)
	data.PricePerMillionCacheRead = types.StringPointerValue(wire.PricePerMillionCacheRead)
	data.PricePerMillionCacheWrite = types.StringPointerValue(wire.PricePerMillionCacheWrite)
	data.CustomContextLength = types.Int64PointerValue(wire.CustomContextLength)
	data.CustomOutputLength = types.Int64PointerValue(wire.CustomOutputLength)
	data.OutputLength = types.Int64PointerValue(wire.OutputLength)
	data.EmbeddingClientImageCapable = types.BoolPointerValue(wire.EmbeddingClientImageCapable)
	data.IsFree = types.BoolPointerValue(wire.IsFree)
	data.SupportsReasoningEffort = types.BoolPointerValue(wire.SupportsReasoningEffort)
	data.ConfiguredParameters = jsontypes.NewNormalizedNull()
	if len(wire.ConfiguredParameters) > 0 && string(wire.ConfiguredParameters) != "null" {
		value, err := normalizedOrganizationJSON(wire.ConfiguredParameters)
		if err != nil {
			diags.AddError("Invalid Model Metadata", "Unable to decode configuredParameters.")
			return
		}
		data.ConfiguredParameters = value
	}
	data.DefaultParameters = jsontypes.NewNormalizedNull()
	if len(wire.DefaultParameters) > 0 && string(wire.DefaultParameters) != "null" {
		value, err := normalizedOrganizationJSON(wire.DefaultParameters)
		if err != nil {
			diags.AddError("Invalid Model Metadata", "Unable to decode defaultParameters.")
			return
		}
		data.DefaultParameters = value
	}
	data.Teams = jsontypes.NewNormalizedNull()
	if len(wire.Teams) > 0 && string(wire.Teams) != "null" {
		value, err := normalizedOrganizationJSON(wire.Teams)
		if err != nil {
			diags.AddError("Invalid Model Metadata", "Unable to decode teams.")
			return
		}
		data.Teams = value
	}
	data.Users = jsontypes.NewNormalizedNull()
	if len(wire.Users) > 0 && string(wire.Users) != "null" {
		value, err := normalizedOrganizationJSON(wire.Users)
		if err != nil {
			diags.AddError("Invalid Model Metadata", "Unable to decode users.")
			return
		}
		data.Users = value
	}
	data.EffectiveContextLength = types.Float64PointerValue(wire.EffectiveContextLength)
	data.SupportedEndpoints = types.ListNull(types.StringType)
	if wire.SupportedEndpoints != nil {
		value, ds := types.ListValueFrom(ctx, types.StringType, wire.SupportedEndpoints)
		diags.Append(ds...)
		data.SupportedEndpoints = value
	}
	data.Labels = flattenAgentLabels(data.Labels, wire.Labels)
	data.EffectiveInputModalities = types.ListNull(types.StringType)
	if wire.InputModalities != nil {
		value, ds := types.ListValueFrom(ctx, types.StringType, wire.InputModalities)
		diags.Append(ds...)
		data.EffectiveInputModalities = value
	}
	data.EffectiveOutputModalities = types.ListNull(types.StringType)
	if wire.OutputModalities != nil {
		value, ds := types.ListValueFrom(ctx, types.StringType, wire.OutputModalities)
		diags.Append(ds...)
		data.EffectiveOutputModalities = value
	}
}
