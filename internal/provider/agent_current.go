package provider

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func currentAgentAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"model_pool_id":                             schema.StringAttribute{Optional: true, MarkdownDescription: "Model pool used for the agent; the API resolves its model and provider key.", Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("model_id"), path.MatchRoot("llm_api_key_id"), path.MatchRoot("llm_model"))}},
		"author_email":                              schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported author email."},
		"created_by":                                schema.StringAttribute{Computed: true, MarkdownDescription: "Creator identity metadata as JSON."},
		"created_by_service_account_id":             schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported created by service account id."},
		"deleted_at":                                schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported deleted at."},
		"last_used_at":                              schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported last used at."},
		"latest_version":                            schema.Int64Attribute{Computed: true, MarkdownDescription: "Server-reported latest version."},
		"activation_skill_mode":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported activation skill mode."},
		"activation_skill_policy_revision":          schema.Int64Attribute{Computed: true, MarkdownDescription: "Server-reported activation skill policy revision."},
		"activation_skills_count":                   schema.Int64Attribute{Computed: true, MarkdownDescription: "Server-reported activation skills count."},
		"llm_provider_requires_per_user_credential": schema.BoolAttribute{Computed: true, MarkdownDescription: "Server-reported llm provider requires per user credential."},
		"resolved_llm_model_name":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported resolved llm model name."},
		"resolved_llm_provider":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported resolved llm provider."},
		"resolved_llm_provider_key_name":            schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported resolved llm provider key name."},
		"sandbox_available":                         schema.BoolAttribute{Computed: true, MarkdownDescription: "Server-reported sandbox available."},
		"access_all_subagents":                      schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether all accessible subagents can be delegated to."},
		"tool_exposure_mode":                        schema.StringAttribute{Optional: true, Computed: true, Validators: []validator.String{stringvalidator.OneOf("full", "search_and_run_only")}, MarkdownDescription: "Tool exposure mode: full or search_and_run_only."},
	}
}
func flattenCurrentAgentFields(data *AgentResourceModel, body []byte, diags *diag.Diagnostics) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(body, &values); err != nil {
		diags.AddError("Invalid Agent Response", "Unable to decode agent metadata.")
		return
	}
	{
		var value *string
		if raw := values["authorEmail"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected authorEmail value.")
				return
			}
		}
		data.AuthorEmail = types.StringPointerValue(value)
	}
	{
		data.CreatedBy = types.StringNull()
		if raw := values["createdBy"]; len(raw) > 0 && string(raw) != "null" {
			var creator map[string]any
			if err := json.Unmarshal(raw, &creator); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected creator metadata.")
				return
			}
			encoded, err := json.Marshal(creator)
			if err != nil {
				diags.AddError("Invalid Agent Response", "Unable to encode creator metadata.")
				return
			}
			data.CreatedBy = types.StringValue(string(encoded))
		}
	}
	{
		var value *string
		if raw := values["createdByServiceAccountId"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected createdByServiceAccountId value.")
				return
			}
		}
		data.CreatedByServiceAccountId = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["deletedAt"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected deletedAt value.")
				return
			}
		}
		data.DeletedAt = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["lastUsedAt"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected lastUsedAt value.")
				return
			}
		}
		data.LastUsedAt = types.StringPointerValue(value)
	}
	{
		var value *int64
		if raw := values["latestVersion"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected latestVersion value.")
				return
			}
		}
		data.LatestVersion = types.Int64PointerValue(value)
	}
	{
		var value *string
		if raw := values["activationSkillMode"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected activationSkillMode value.")
				return
			}
		}
		data.ActivationSkillMode = types.StringPointerValue(value)
	}
	{
		var value *int64
		if raw := values["activationSkillPolicyRevision"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected activationSkillPolicyRevision value.")
				return
			}
		}
		data.ActivationSkillPolicyRevision = types.Int64PointerValue(value)
	}
	{
		var value *int64
		if raw := values["activationSkillsCount"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected activationSkillsCount value.")
				return
			}
		}
		data.ActivationSkillsCount = types.Int64PointerValue(value)
	}
	{
		var value *bool
		if raw := values["llmProviderRequiresPerUserCredential"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected llmProviderRequiresPerUserCredential value.")
				return
			}
		}
		data.LlmProviderRequiresPerUserCredential = types.BoolPointerValue(value)
	}
	{
		var value *string
		if raw := values["resolvedLlmModelName"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected resolvedLlmModelName value.")
				return
			}
		}
		data.ResolvedLlmModelName = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["resolvedLlmProvider"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected resolvedLlmProvider value.")
				return
			}
		}
		data.ResolvedLlmProvider = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["resolvedLlmProviderKeyName"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected resolvedLlmProviderKeyName value.")
				return
			}
		}
		data.ResolvedLlmProviderKeyName = types.StringPointerValue(value)
	}
	{
		var value *bool
		if raw := values["sandboxAvailable"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected sandboxAvailable value.")
				return
			}
		}
		data.SandboxAvailable = types.BoolPointerValue(value)
	}
	{
		var value *bool
		if raw := values["accessAllSubagents"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected accessAllSubagents value.")
				return
			}
		}
		data.AccessAllSubagents = types.BoolPointerValue(value)
	}
	{
		var value *string
		if raw := values["toolExposureMode"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected toolExposureMode value.")
				return
			}
		}
		data.ToolExposureMode = types.StringPointerValue(value)
	}
}
func currentGatewayAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"model_pool_id":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported model pool identifier on the shared agent record."},
		"github_actions_access":         githubActionsAccessSchema(),
		"author_email":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported author email."},
		"created_by":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Creator identity metadata as JSON."},
		"created_by_service_account_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported created by service account id."},
		"deleted_at":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported deleted at."},
		"last_used_at":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Server-reported last used at."},
		"latest_version":                schema.Int64Attribute{Computed: true, MarkdownDescription: "Server-reported latest version."},
		"access_all_tools":              schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether all accessible tools are exposed automatically."},
		"missing_credential_behavior":   schema.StringAttribute{Optional: true, Computed: true, Validators: []validator.String{stringvalidator.OneOf("allow", "warn", "block")}, MarkdownDescription: "Behavior when a caller has no usable tool credential."},
		"environment_id":                schema.StringAttribute{Optional: true, MarkdownDescription: "Deployment environment used to resolve MCP servers and credentials."},
	}
}
func flattenCurrentGatewayFields(data *McpGatewayResourceModel, body []byte, diags *diag.Diagnostics) {
	var pool struct {
		ID *string `json:"modelPoolId"`
	}
	if err := json.Unmarshal(body, &pool); err != nil {
		diags.AddError("Invalid Gateway Response", err.Error())
		return
	}
	data.ModelPoolID = types.StringPointerValue(pool.ID)
	data.GithubActionsAccess = githubActionsAccessFromResponse(body, diags)
	var values map[string]json.RawMessage
	if err := json.Unmarshal(body, &values); err != nil {
		diags.AddError("Invalid Agent Response", "Unable to decode agent metadata.")
		return
	}
	{
		var value *string
		if raw := values["authorEmail"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected authorEmail value.")
				return
			}
		}
		data.AuthorEmail = types.StringPointerValue(value)
	}
	{
		data.CreatedBy = types.StringNull()
		if raw := values["createdBy"]; len(raw) > 0 && string(raw) != "null" {
			var creator map[string]any
			if err := json.Unmarshal(raw, &creator); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected creator metadata.")
				return
			}
			encoded, err := json.Marshal(creator)
			if err != nil {
				diags.AddError("Invalid Agent Response", "Unable to encode creator metadata.")
				return
			}
			data.CreatedBy = types.StringValue(string(encoded))
		}
	}
	{
		var value *string
		if raw := values["createdByServiceAccountId"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected createdByServiceAccountId value.")
				return
			}
		}
		data.CreatedByServiceAccountId = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["deletedAt"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected deletedAt value.")
				return
			}
		}
		data.DeletedAt = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["lastUsedAt"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected lastUsedAt value.")
				return
			}
		}
		data.LastUsedAt = types.StringPointerValue(value)
	}
	{
		var value *int64
		if raw := values["latestVersion"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected latestVersion value.")
				return
			}
		}
		data.LatestVersion = types.Int64PointerValue(value)
	}
	{
		var value *bool
		if raw := values["accessAllTools"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected accessAllTools value.")
				return
			}
		}
		data.AccessAllTools = types.BoolPointerValue(value)
	}
	{
		var value *string
		if raw := values["missingCredentialBehavior"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected missingCredentialBehavior value.")
				return
			}
		}
		data.MissingCredentialBehavior = types.StringPointerValue(value)
	}
	{
		var value *string
		if raw := values["environmentId"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &value); err != nil {
				diags.AddError("Invalid Agent Response", "Unexpected environmentId value.")
				return
			}
		}
		optionalStringFromAPI(&data.EnvironmentId, value)
	}
}
