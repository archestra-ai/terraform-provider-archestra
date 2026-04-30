package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// agentAPIResponse mirrors the JSON shape of an agent record from the
// `/api/agents` endpoints. The generated client uses inline anonymous structs
// per response type, so this internal mirror lets shared flatten helpers
// operate on the raw body bytes regardless of which endpoint produced them.
type agentAPIResponse struct {
	Id                         openapi_types.UUID  `json:"id"`
	Name                       string              `json:"name"`
	AgentType                  string              `json:"agentType"`
	Description                *string             `json:"description"`
	Icon                       *string             `json:"icon"`
	SystemPrompt               *string             `json:"systemPrompt"`
	LlmModel                   *string             `json:"llmModel"`
	LlmApiKeyId                *openapi_types.UUID `json:"llmApiKeyId"`
	IdentityProviderId         *string             `json:"identityProviderId"`
	IncomingEmailEnabled       bool                `json:"incomingEmailEnabled"`
	IncomingEmailAllowedDomain *string             `json:"incomingEmailAllowedDomain"`
	IncomingEmailSecurityMode  string              `json:"incomingEmailSecurityMode"`
	ConsiderContextUntrusted   bool                `json:"considerContextUntrusted"`
	IsDefault                  bool                `json:"isDefault"`
	Scope                      string              `json:"scope"`
	PassthroughHeaders         *[]string           `json:"passthroughHeaders"`
	KnowledgeBaseIds           []string            `json:"knowledgeBaseIds"`
	ConnectorIds               []string            `json:"connectorIds"`
	Teams                      []struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"teams"`
	Labels []struct {
		Key     string              `json:"key"`
		KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
		Value   string              `json:"value"`
		ValueId *openapi_types.UUID `json:"valueId,omitempty"`
	} `json:"labels"`
	SuggestedPrompts []struct {
		Prompt       string `json:"prompt"`
		SummaryTitle string `json:"summaryTitle"`
	} `json:"suggestedPrompts"`
	BuiltInAgentConfig *json.RawMessage `json:"builtInAgentConfig"`
}

// parseAgentResponse decodes a raw API response body into the shared agent
// shape. Returns nil and a diagnostic on failure.
func parseAgentResponse(body []byte, diags *diag.Diagnostics) *agentAPIResponse {
	var out agentAPIResponse
	if err := json.Unmarshal(body, &out); err != nil {
		diags.AddError("API Response Decode Error", "Unable to decode agent response: "+err.Error())
		return nil
	}
	return &out
}

// AgentLabelModel describes a label key/value pair on an agent (any type).
type AgentLabelModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

// SuggestedPromptModel describes a suggested prompt on an internal agent.
type SuggestedPromptModel struct {
	Prompt       types.String `tfsdk:"prompt"`
	SummaryTitle types.String `tfsdk:"summary_title"`
}

// BuiltInAgentConfigModel describes the built-in agent configuration block.
type BuiltInAgentConfigModel struct {
	Name                         types.String `tfsdk:"name"`
	AutoConfigureOnToolDiscovery types.Bool   `tfsdk:"auto_configure_on_tool_discovery"`
	MaxRounds                    types.Int64  `tfsdk:"max_rounds"`
}

// flattenAgentLabels copies API-side labels into the model. Labels are exposed
// as a set in the schema, so element order is irrelevant for state comparison
// — both apply and import paths can return whatever order the API gives us.
//
// `prior` distinguishes two empty-API cases that produce different states:
// a nil prior (HCL omits the block) collapses to a typed-null set, while a
// non-nil empty prior (HCL writes `labels = []`) preserves the explicit
// empty so refresh doesn't churn null↔[] forever.
func flattenAgentLabels(prior []AgentLabelModel, apiLabels []struct {
	Key     string              `json:"key"`
	KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
	Value   string              `json:"value"`
	ValueId *openapi_types.UUID `json:"valueId,omitempty"`
}) []AgentLabelModel {
	if len(apiLabels) == 0 {
		if prior != nil {
			return []AgentLabelModel{}
		}
		return nil
	}
	out := make([]AgentLabelModel, len(apiLabels))
	for i, l := range apiLabels {
		out[i] = AgentLabelModel{Key: types.StringValue(l.Key), Value: types.StringValue(l.Value)}
	}
	return out
}

// stringListFromAPI updates a types.List target from a string slice returned
// by the API, normalising empty results back to a typed null list.
func stringListFromAPI(ctx context.Context, target *types.List, apiValues []string, diags *diag.Diagnostics) {
	if len(apiValues) > 0 {
		list, d := types.ListValueFrom(ctx, types.StringType, apiValues)
		diags.Append(d...)
		*target = list
		return
	}
	if !target.IsNull() {
		*target = types.ListNull(types.StringType)
	}
}

// teamsListFromAPI builds a list of team IDs from the API agent response.
// `prior` preserves the null-vs-[] distinction across refresh — see
// flattenAgentLabels for the rationale.
func teamsListFromAPI(ctx context.Context, prior types.List, teams []struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}, diags *diag.Diagnostics) types.List {
	if len(teams) == 0 && prior.IsNull() {
		return types.ListNull(types.StringType)
	}
	ids := make([]string, len(teams))
	for i, t := range teams {
		ids[i] = t.Id
	}
	list, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	return list
}

// optionalStringFromAPI updates a types.String target from an *string returned
// by the API, normalising nil to typed null.
func optionalStringFromAPI(target *types.String, v *string) {
	if v != nil {
		*target = types.StringValue(*v)
		return
	}
	if !target.IsNull() {
		*target = types.StringNull()
	}
}

// optionalUUIDFromAPI updates a types.String target from an *openapi_types.UUID.
func optionalUUIDFromAPI(target *types.String, v *openapi_types.UUID) {
	if v != nil {
		*target = types.StringValue(v.String())
		return
	}
	if !target.IsNull() {
		*target = types.StringNull()
	}
}

// suggestedPromptsToAPI converts the model representation to the API wire
// format for suggestedPrompts.
// suggestedPromptsFromAPI maps suggestedPrompts from API response into model.
func suggestedPromptsFromAPI(apiPrompts []struct {
	Prompt       string `json:"prompt"`
	SummaryTitle string `json:"summaryTitle"`
}) []SuggestedPromptModel {
	if len(apiPrompts) == 0 {
		return nil
	}
	out := make([]SuggestedPromptModel, len(apiPrompts))
	for i, sp := range apiPrompts {
		out[i] = SuggestedPromptModel{
			Prompt:       types.StringValue(sp.Prompt),
			SummaryTitle: types.StringValue(sp.SummaryTitle),
		}
	}
	return out
}

// builtInAgentConfigFromResponse parses builtInAgentConfig from a raw API
// response body and returns a populated model (or nil if absent/invalid).
func builtInAgentConfigFromResponse(responseBody []byte) *BuiltInAgentConfigModel {
	var rawResp struct {
		BuiltInAgentConfig *json.RawMessage `json:"builtInAgentConfig"`
	}
	if err := json.Unmarshal(responseBody, &rawResp); err != nil || rawResp.BuiltInAgentConfig == nil {
		return nil
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(*rawResp.BuiltInAgentConfig, &configMap); err != nil {
		return nil
	}

	name, _ := configMap["name"].(string)
	if name == "" {
		return nil
	}

	out := &BuiltInAgentConfigModel{
		Name:                         types.StringValue(name),
		AutoConfigureOnToolDiscovery: types.BoolNull(),
		MaxRounds:                    types.Int64Null(),
	}
	switch name {
	case "policy-configuration-subagent":
		if v, ok := configMap["autoConfigureOnToolDiscovery"].(bool); ok {
			out.AutoConfigureOnToolDiscovery = types.BoolValue(v)
		}
	case "dual-llm-main-agent":
		if v, ok := configMap["maxRounds"].(float64); ok {
			out.MaxRounds = types.Int64Value(int64(v))
		}
	}
	return out
}
