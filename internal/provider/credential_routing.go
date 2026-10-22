package provider

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var credentialProviderKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"provider": types.StringType, "provider_api_key_id": types.StringType,
}}
var credentialLabelType = types.ObjectType{AttrTypes: map[string]attr.Type{"key": types.StringType, "value": types.StringType}}
var credentialSpendCapTypes = map[string]attr.Type{
	"limit_value": types.Int64Type, "cleanup_interval": types.StringType,
}
var credentialProviderKeySpec = AttrSpec{TFName: "provider_api_keys", JSONName: "providerApiKeys", Kind: Set, OmitOnNull: true, Children: []AttrSpec{
	{TFName: "provider", JSONName: "provider", Kind: Scalar},
	{TFName: "provider_api_key_id", JSONName: "providerApiKeyId", Kind: Scalar},
}}
var credentialLabelsSpec = AttrSpec{TFName: "labels", JSONName: "labels", Kind: Set, OmitOnNull: true, Children: []AttrSpec{
	{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar},
}}
var credentialSpendCapSpec = AttrSpec{TFName: "spend_cap", JSONName: "spendCap", Kind: AtomicObject, Children: []AttrSpec{
	{TFName: "limit_value", JSONName: "limitValue", Kind: Scalar},
	{TFName: "cleanup_interval", JSONName: "cleanupInterval", Kind: Scalar},
}}

func credentialProviderKeysAttribute() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{Optional: true, Computed: true, MarkdownDescription: "Explicit provider-to-key mappings; an empty set clears explicit mappings.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"provider":            schema.StringAttribute{Required: true, MarkdownDescription: "LLM provider identifier."},
		"provider_api_key_id": schema.StringAttribute{Required: true, MarkdownDescription: "Existing provider API key UUID."},
	}}}
}
func credentialLabelsAttribute() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{Optional: true, Computed: true, MarkdownDescription: "Credential labels; an empty set clears them.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"key": schema.StringAttribute{Required: true}, "value": schema.StringAttribute{Required: true},
	}}}
}
func credentialSpendCapAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Credential-wide spend cap; removing it clears the cap.", Attributes: map[string]schema.Attribute{
		"limit_value":      schema.Int64Attribute{Required: true, MarkdownDescription: "Positive dollar limit per window."},
		"cleanup_interval": schema.StringAttribute{Required: true, MarkdownDescription: "Limit cleanup interval."},
	}}
}

type credentialProviderKeyModel struct {
	Provider         types.String `tfsdk:"provider"`
	ProviderAPIKeyID types.String `tfsdk:"provider_api_key_id"`
}
type credentialRoutingResponse struct {
	ProviderAPIKeys []struct {
		Provider string `json:"provider"`
		ID       string `json:"providerApiKeyId"`
	} `json:"providerApiKeys"`
	Labels []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"labels"`
	BillingTeam *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"billingTeam"`
	SpendCap *struct {
		LimitValue      int64  `json:"limitValue"`
		CleanupInterval string `json:"cleanupInterval"`
	} `json:"spendCap"`
}

func flattenCredentialRouting(ctx context.Context, raw credentialRoutingResponse, providers, labels *types.Set, spend *types.Object, billingID, billingName *types.String, diags *diag.Diagnostics) {
	keys := make([]credentialProviderKeyModel, len(raw.ProviderAPIKeys))
	for i, key := range raw.ProviderAPIKeys {
		keys[i] = credentialProviderKeyModel{Provider: types.StringValue(key.Provider), ProviderAPIKeyID: types.StringValue(key.ID)}
	}
	var d diag.Diagnostics
	*providers, d = types.SetValueFrom(ctx, credentialProviderKeyType, keys)
	diags.Append(d...)
	labelValues := make([]AgentLabelModel, len(raw.Labels))
	for i, label := range raw.Labels {
		labelValues[i] = AgentLabelModel{Key: types.StringValue(label.Key), Value: types.StringValue(label.Value)}
	}
	*labels, d = types.SetValueFrom(ctx, credentialLabelType, labelValues)
	diags.Append(d...)
	*spend = types.ObjectNull(credentialSpendCapTypes)
	if budget := raw.SpendCap; budget != nil {
		*spend, d = types.ObjectValue(credentialSpendCapTypes, map[string]attr.Value{
			"limit_value": types.Int64Value(budget.LimitValue), "cleanup_interval": types.StringValue(budget.CleanupInterval),
		})
		diags.Append(d...)
	}
	*billingID = types.StringNull()
	*billingName = types.StringNull()
	if raw.BillingTeam != nil {
		*billingID = types.StringValue(raw.BillingTeam.ID)
		*billingName = types.StringValue(raw.BillingTeam.Name)
	}
}
func credentialProviderRequest(ctx context.Context, value types.Set, diags *diag.Diagnostics) []map[string]any {
	var keys []credentialProviderKeyModel
	diags.Append(value.ElementsAs(ctx, &keys, false)...)
	result := make([]map[string]any, len(keys))
	for i, key := range keys {
		result[i] = map[string]any{"provider": key.Provider.ValueString(), "providerApiKeyId": key.ProviderAPIKeyID.ValueString()}
	}
	return result
}
func credentialJSONValue(raw json.RawMessage, diags *diag.Diagnostics) types.String {
	value, err := normalizedOrganizationJSON(raw)
	if err != nil {
		diags.AddError("Invalid API metadata", err.Error())
		return types.StringNull()
	}
	if value.IsNull() {
		return types.StringNull()
	}
	return types.StringValue(value.ValueString())
}
