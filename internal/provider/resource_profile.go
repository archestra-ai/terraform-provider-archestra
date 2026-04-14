package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ProfileResource{}
var _ resource.ResourceWithImportState = &ProfileResource{}

func NewProfileResource() resource.Resource {
	return &ProfileResource{}
}

// ProfileResource defines the resource implementation.
type ProfileResource struct {
	client *client.ClientWithResponses
}

// ProfileLabelModel describes a label data model.
type ProfileLabelModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

// SuggestedPromptModel describes a suggested prompt data model.
type SuggestedPromptModel struct {
	Prompt       types.String `tfsdk:"prompt"`
	SummaryTitle types.String `tfsdk:"summary_title"`
}

// BuiltInAgentConfigModel describes the built-in agent configuration.
type BuiltInAgentConfigModel struct {
	Name                         types.String `tfsdk:"name"`
	AutoConfigureOnToolDiscovery types.Bool   `tfsdk:"auto_configure_on_tool_discovery"`
	MaxRounds                    types.Int64  `tfsdk:"max_rounds"`
}

// ProfileResourceModel describes the resource data model.
type ProfileResourceModel struct {
	ID                         types.String             `tfsdk:"id"`
	Name                       types.String             `tfsdk:"name"`
	Description                types.String             `tfsdk:"description"`
	Icon                       types.String             `tfsdk:"icon"`
	SystemPrompt               types.String             `tfsdk:"system_prompt"`
	LlmModel                   types.String             `tfsdk:"llm_model"`
	LlmApiKeyId                types.String             `tfsdk:"llm_api_key_id"`
	AgentType                  types.String             `tfsdk:"agent_type"`
	PassthroughHeaders         types.List               `tfsdk:"passthrough_headers"`
	KnowledgeBaseIds           types.List               `tfsdk:"knowledge_base_ids"`
	ConnectorIds               types.List               `tfsdk:"connector_ids"`
	IncomingEmailEnabled       types.Bool               `tfsdk:"incoming_email_enabled"`
	IncomingEmailAllowedDomain types.String             `tfsdk:"incoming_email_allowed_domain"`
	IncomingEmailSecurityMode  types.String             `tfsdk:"incoming_email_security_mode"`
	ConsiderContextUntrusted   types.Bool               `tfsdk:"consider_context_untrusted"`
	IsDefault                  types.Bool               `tfsdk:"is_default"`
	IdentityProviderId         types.String             `tfsdk:"identity_provider_id"`
	SuggestedPrompts           []SuggestedPromptModel   `tfsdk:"suggested_prompts"`
	Labels                     []ProfileLabelModel      `tfsdk:"labels"`
	BuiltInAgentConfig         *BuiltInAgentConfigModel `tfsdk:"built_in_agent_config"`
}

func (r *ProfileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile"
}

func (r *ProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra profile.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Profile identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the profile",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the profile",
				Optional:            true,
			},
			"icon": schema.StringAttribute{
				MarkdownDescription: "Emoji or base64 image for the profile icon",
				Optional:            true,
			},
			"system_prompt": schema.StringAttribute{
				MarkdownDescription: "System prompt for agent-type agents",
				Optional:            true,
			},
			"llm_model": schema.StringAttribute{
				MarkdownDescription: "LLM model ID",
				Optional:            true,
			},
			"llm_api_key_id": schema.StringAttribute{
				MarkdownDescription: "LLM API key UUID",
				Optional:            true,
			},
			"agent_type": schema.StringAttribute{
				MarkdownDescription: "The type of the agent. Valid values: profile, mcp_gateway, llm_proxy, agent",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"passthrough_headers": schema.ListAttribute{
				MarkdownDescription: "HTTP headers to forward",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"knowledge_base_ids": schema.ListAttribute{
				MarkdownDescription: "List of knowledge base IDs to associate with the profile",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"connector_ids": schema.ListAttribute{
				MarkdownDescription: "List of connector IDs to associate with the profile",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"incoming_email_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable email trigger",
				Optional:            true,
				Computed:            true,
			},
			"incoming_email_allowed_domain": schema.StringAttribute{
				MarkdownDescription: "Domain for internal email security mode",
				Optional:            true,
			},
			"incoming_email_security_mode": schema.StringAttribute{
				MarkdownDescription: "Email security mode: private, internal, or public",
				Optional:            true,
				Computed:            true,
			},
			"consider_context_untrusted": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent context is treated as untrusted",
				Optional:            true,
				Computed:            true,
			},
			"is_default": schema.BoolAttribute{
				MarkdownDescription: "Whether this is the default agent",
				Optional:            true,
				Computed:            true,
			},
			"identity_provider_id": schema.StringAttribute{
				MarkdownDescription: "Identity provider ID for SSO",
				Optional:            true,
			},
			"suggested_prompts": schema.ListNestedAttribute{
				MarkdownDescription: "Suggested prompts for the profile",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"prompt": schema.StringAttribute{
							MarkdownDescription: "The prompt text",
							Required:            true,
						},
						"summary_title": schema.StringAttribute{
							MarkdownDescription: "The summary title for the prompt",
							Required:            true,
						},
					},
				},
			},
			"labels": schema.ListNestedAttribute{
				MarkdownDescription: "Labels to organize and identify the profile",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Label key",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Label value",
							Required:            true,
						},
					},
				},
			},
		},

		Blocks: map[string]schema.Block{
			"built_in_agent_config": schema.SingleNestedBlock{
				MarkdownDescription: "Built-in agent configuration. Discriminated by `name`.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						MarkdownDescription: "The built-in agent name. Valid values: `policy-configuration-subagent`, `dual-llm-main-agent`, `dual-llm-quarantine-agent`.",
						Optional:            true,
					},
					"auto_configure_on_tool_discovery": schema.BoolAttribute{
						MarkdownDescription: "Whether to auto-configure on tool discovery. Only applicable when `name` is `policy-configuration-subagent`.",
						Optional:            true,
					},
					"max_rounds": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of rounds (1-20). Only applicable when `name` is `dual-llm-main-agent`.",
						Optional:            true,
						Computed:            true,
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.UseStateForUnknown(),
						},
					},
				},
			},
		},
	}
}

func (r *ProfileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *ProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProfileResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert labels to API format (initialize as empty slice to avoid null in JSON)
	labels := make([]struct {
		Key     string              `json:"key"`
		KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
		Value   string              `json:"value"`
		ValueId *openapi_types.UUID `json:"valueId,omitempty"`
	}, 0)

	for _, label := range data.Labels {
		labels = append(labels, struct {
			Key     string              `json:"key"`
			KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
			Value   string              `json:"value"`
			ValueId *openapi_types.UUID `json:"valueId,omitempty"`
		}{
			Key:   label.Key.ValueString(),
			Value: label.Value.ValueString(),
		})
	}

	// Create request body using generated type
	emptyTeams := []string{}
	requestBody := client.CreateAgentJSONRequestBody{
		Name:   data.Name.ValueString(),
		Scope:  client.CreateAgentJSONBodyScopeOrg,
		Teams:  &emptyTeams,
		Labels: &labels,
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		desc := data.Description.ValueString()
		requestBody.Description = &desc
	}
	if !data.Icon.IsNull() && !data.Icon.IsUnknown() {
		icon := data.Icon.ValueString()
		requestBody.Icon = &icon
	}
	if !data.SystemPrompt.IsNull() && !data.SystemPrompt.IsUnknown() {
		sp := data.SystemPrompt.ValueString()
		requestBody.SystemPrompt = &sp
	}
	if !data.LlmModel.IsNull() && !data.LlmModel.IsUnknown() {
		m := data.LlmModel.ValueString()
		requestBody.LlmModel = &m
	}
	if !data.LlmApiKeyId.IsNull() && !data.LlmApiKeyId.IsUnknown() {
		id, parseErr := uuid.Parse(data.LlmApiKeyId.ValueString())
		if parseErr != nil {
			resp.Diagnostics.AddError("Invalid llm_api_key_id", fmt.Sprintf("Unable to parse llm_api_key_id: %s", parseErr))
			return
		}
		requestBody.LlmApiKeyId = &id
	}
	if !data.PassthroughHeaders.IsNull() && !data.PassthroughHeaders.IsUnknown() {
		var headers []string
		resp.Diagnostics.Append(data.PassthroughHeaders.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.PassthroughHeaders = &headers
	}
	if !data.IncomingEmailEnabled.IsNull() && !data.IncomingEmailEnabled.IsUnknown() {
		enabled := data.IncomingEmailEnabled.ValueBool()
		requestBody.IncomingEmailEnabled = &enabled
	}
	if !data.IncomingEmailAllowedDomain.IsNull() && !data.IncomingEmailAllowedDomain.IsUnknown() {
		domain := data.IncomingEmailAllowedDomain.ValueString()
		requestBody.IncomingEmailAllowedDomain = &domain
	}
	if !data.IncomingEmailSecurityMode.IsNull() && !data.IncomingEmailSecurityMode.IsUnknown() {
		mode := client.CreateAgentJSONBodyIncomingEmailSecurityMode(data.IncomingEmailSecurityMode.ValueString())
		requestBody.IncomingEmailSecurityMode = &mode
	}
	if !data.ConsiderContextUntrusted.IsNull() && !data.ConsiderContextUntrusted.IsUnknown() {
		v := data.ConsiderContextUntrusted.ValueBool()
		requestBody.ConsiderContextUntrusted = &v
	}
	if !data.IsDefault.IsNull() && !data.IsDefault.IsUnknown() {
		v := data.IsDefault.ValueBool()
		requestBody.IsDefault = &v
	}
	if !data.IdentityProviderId.IsNull() && !data.IdentityProviderId.IsUnknown() {
		v := data.IdentityProviderId.ValueString()
		requestBody.IdentityProviderId = &v
	}
	if !data.AgentType.IsNull() && !data.AgentType.IsUnknown() {
		at := client.CreateAgentJSONBodyAgentType(data.AgentType.ValueString())
		requestBody.AgentType = &at
	}
	if data.SuggestedPrompts != nil {
		prompts := make([]struct {
			Prompt       string `json:"prompt"`
			SummaryTitle string `json:"summaryTitle"`
		}, len(data.SuggestedPrompts))
		for i, sp := range data.SuggestedPrompts {
			prompts[i] = struct {
				Prompt       string `json:"prompt"`
				SummaryTitle string `json:"summaryTitle"`
			}{
				Prompt:       sp.Prompt.ValueString(),
				SummaryTitle: sp.SummaryTitle.ValueString(),
			}
		}
		requestBody.SuggestedPrompts = &prompts
	}
	if !data.KnowledgeBaseIds.IsNull() && !data.KnowledgeBaseIds.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.KnowledgeBaseIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.KnowledgeBaseIds = &ids
	}
	if !data.ConnectorIds.IsNull() && !data.ConnectorIds.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.ConnectorIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.ConnectorIds = &ids
	}

	// Call API
	var apiResp *client.CreateAgentResponse
	if data.BuiltInAgentConfig != nil && !data.BuiltInAgentConfig.Name.IsNull() {
		// Marshal base request body, inject builtInAgentConfig, and send as raw JSON
		bodyBytes, marshalErr := json.Marshal(requestBody)
		if marshalErr != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", marshalErr))
			return
		}
		configJSON := r.buildBuiltInAgentConfigJSON(data.BuiltInAgentConfig)
		bodyBytes, marshalErr = r.injectBuiltInAgentConfig(bodyBytes, configJSON)
		if marshalErr != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to inject builtInAgentConfig: %s", marshalErr))
			return
		}
		var createErr error
		apiResp, createErr = r.client.CreateAgentWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
		if createErr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create profile, got error: %s", createErr))
			return
		}
	} else {
		var createErr error
		apiResp, createErr = r.client.CreateAgentWithResponse(ctx, requestBody)
		if createErr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create profile, got error: %s", createErr))
			return
		}
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.Name = types.StringValue(apiResp.JSON200.Name)
	r.mapResponseFieldsToState(&data, apiResp.JSON200.Description, apiResp.JSON200.Icon, apiResp.JSON200.SystemPrompt, apiResp.JSON200.LlmModel, apiResp.JSON200.LlmApiKeyId, apiResp.JSON200.PassthroughHeaders, apiResp.JSON200.IncomingEmailEnabled, apiResp.JSON200.IncomingEmailAllowedDomain, string(apiResp.JSON200.IncomingEmailSecurityMode), ctx, resp.Diagnostics)

	// Map agent_type from response
	data.AgentType = types.StringValue(string(apiResp.JSON200.AgentType))

	// Map consider_context_untrusted and is_default from response (non-nullable bools)
	data.ConsiderContextUntrusted = types.BoolValue(apiResp.JSON200.ConsiderContextUntrusted)
	data.IsDefault = types.BoolValue(apiResp.JSON200.IsDefault)

	// Map identity_provider_id from response (nullable)
	if apiResp.JSON200.IdentityProviderId != nil {
		data.IdentityProviderId = types.StringValue(*apiResp.JSON200.IdentityProviderId)
	} else if !data.IdentityProviderId.IsNull() {
		data.IdentityProviderId = types.StringNull()
	}

	// Map suggested_prompts from response
	r.mapSuggestedPromptsToState(&data, apiResp.JSON200.SuggestedPrompts)

	// Map knowledge_base_ids from response
	r.mapStringListToState(ctx, &data.KnowledgeBaseIds, apiResp.JSON200.KnowledgeBaseIds, resp.Diagnostics)

	// Map connector_ids from response
	r.mapStringListToState(ctx, &data.ConnectorIds, apiResp.JSON200.ConnectorIds, resp.Diagnostics)

	// Map labels from API response, preserving configuration order
	// If labels were not specified in config (nil), keep them nil in state
	if data.Labels != nil {
		data.Labels = r.mapLabelsToConfigurationOrder(data.Labels, apiResp.JSON200.Labels)
	}

	// Map built_in_agent_config from response
	r.mapBuiltInAgentConfigToState(&data, apiResp.Body)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProfileResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	profileID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.GetAgentWithResponse(ctx, profileID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read profile, got error: %s", err))
		return
	}

	// Handle not found
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)
	r.mapResponseFieldsToState(&data, apiResp.JSON200.Description, apiResp.JSON200.Icon, apiResp.JSON200.SystemPrompt, apiResp.JSON200.LlmModel, apiResp.JSON200.LlmApiKeyId, apiResp.JSON200.PassthroughHeaders, apiResp.JSON200.IncomingEmailEnabled, apiResp.JSON200.IncomingEmailAllowedDomain, string(apiResp.JSON200.IncomingEmailSecurityMode), ctx, resp.Diagnostics)

	// Map agent_type from response
	data.AgentType = types.StringValue(string(apiResp.JSON200.AgentType))

	// Map consider_context_untrusted and is_default from response (non-nullable bools)
	data.ConsiderContextUntrusted = types.BoolValue(apiResp.JSON200.ConsiderContextUntrusted)
	data.IsDefault = types.BoolValue(apiResp.JSON200.IsDefault)

	// Map identity_provider_id from response (nullable)
	if apiResp.JSON200.IdentityProviderId != nil {
		data.IdentityProviderId = types.StringValue(*apiResp.JSON200.IdentityProviderId)
	} else if !data.IdentityProviderId.IsNull() {
		data.IdentityProviderId = types.StringNull()
	}

	// Map suggested_prompts from response
	r.mapSuggestedPromptsToState(&data, apiResp.JSON200.SuggestedPrompts)

	// Map knowledge_base_ids from response
	r.mapStringListToState(ctx, &data.KnowledgeBaseIds, apiResp.JSON200.KnowledgeBaseIds, resp.Diagnostics)

	// Map connector_ids from response
	r.mapStringListToState(ctx, &data.ConnectorIds, apiResp.JSON200.ConnectorIds, resp.Diagnostics)

	// Map labels from API response, preserving existing state order
	// If labels were not specified in state (nil), keep them nil
	if data.Labels != nil {
		data.Labels = r.mapLabelsToConfigurationOrder(data.Labels, apiResp.JSON200.Labels)
	}

	// Map built_in_agent_config from response
	r.mapBuiltInAgentConfigToState(&data, apiResp.Body)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProfileResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	profileID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	// Convert labels to API format (initialize as empty slice to avoid null in JSON)
	labels := make([]struct {
		Key     string              `json:"key"`
		KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
		Value   string              `json:"value"`
		ValueId *openapi_types.UUID `json:"valueId,omitempty"`
	}, 0)

	for _, label := range data.Labels {
		labels = append(labels, struct {
			Key     string              `json:"key"`
			KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
			Value   string              `json:"value"`
			ValueId *openapi_types.UUID `json:"valueId,omitempty"`
		}{
			Key:   label.Key.ValueString(),
			Value: label.Value.ValueString(),
		})
	}

	// Create request body using generated type
	name := data.Name.ValueString()
	requestBody := client.UpdateAgentJSONRequestBody{
		Name:   &name,
		Labels: &labels,
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		desc := data.Description.ValueString()
		requestBody.Description = &desc
	}
	if !data.Icon.IsNull() && !data.Icon.IsUnknown() {
		icon := data.Icon.ValueString()
		requestBody.Icon = &icon
	}
	if !data.SystemPrompt.IsNull() && !data.SystemPrompt.IsUnknown() {
		sp := data.SystemPrompt.ValueString()
		requestBody.SystemPrompt = &sp
	}
	if !data.LlmModel.IsNull() && !data.LlmModel.IsUnknown() {
		m := data.LlmModel.ValueString()
		requestBody.LlmModel = &m
	}
	if !data.LlmApiKeyId.IsNull() && !data.LlmApiKeyId.IsUnknown() {
		id, parseErr := uuid.Parse(data.LlmApiKeyId.ValueString())
		if parseErr != nil {
			resp.Diagnostics.AddError("Invalid llm_api_key_id", fmt.Sprintf("Unable to parse llm_api_key_id: %s", parseErr))
			return
		}
		requestBody.LlmApiKeyId = &id
	}
	if !data.PassthroughHeaders.IsNull() && !data.PassthroughHeaders.IsUnknown() {
		var headers []string
		resp.Diagnostics.Append(data.PassthroughHeaders.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.PassthroughHeaders = &headers
	}
	if !data.IncomingEmailEnabled.IsNull() && !data.IncomingEmailEnabled.IsUnknown() {
		enabled := data.IncomingEmailEnabled.ValueBool()
		requestBody.IncomingEmailEnabled = &enabled
	}
	if !data.IncomingEmailAllowedDomain.IsNull() && !data.IncomingEmailAllowedDomain.IsUnknown() {
		domain := data.IncomingEmailAllowedDomain.ValueString()
		requestBody.IncomingEmailAllowedDomain = &domain
	}
	if !data.IncomingEmailSecurityMode.IsNull() && !data.IncomingEmailSecurityMode.IsUnknown() {
		mode := client.UpdateAgentJSONBodyIncomingEmailSecurityMode(data.IncomingEmailSecurityMode.ValueString())
		requestBody.IncomingEmailSecurityMode = &mode
	}
	if !data.ConsiderContextUntrusted.IsNull() && !data.ConsiderContextUntrusted.IsUnknown() {
		v := data.ConsiderContextUntrusted.ValueBool()
		requestBody.ConsiderContextUntrusted = &v
	}
	if !data.IsDefault.IsNull() && !data.IsDefault.IsUnknown() {
		v := data.IsDefault.ValueBool()
		requestBody.IsDefault = &v
	}
	if !data.IdentityProviderId.IsNull() && !data.IdentityProviderId.IsUnknown() {
		v := data.IdentityProviderId.ValueString()
		requestBody.IdentityProviderId = &v
	}
	if !data.AgentType.IsNull() && !data.AgentType.IsUnknown() {
		at := client.UpdateAgentJSONBodyAgentType(data.AgentType.ValueString())
		requestBody.AgentType = &at
	}
	if data.SuggestedPrompts != nil {
		prompts := make([]struct {
			Prompt       string `json:"prompt"`
			SummaryTitle string `json:"summaryTitle"`
		}, len(data.SuggestedPrompts))
		for i, sp := range data.SuggestedPrompts {
			prompts[i] = struct {
				Prompt       string `json:"prompt"`
				SummaryTitle string `json:"summaryTitle"`
			}{
				Prompt:       sp.Prompt.ValueString(),
				SummaryTitle: sp.SummaryTitle.ValueString(),
			}
		}
		requestBody.SuggestedPrompts = &prompts
	}
	if !data.KnowledgeBaseIds.IsNull() && !data.KnowledgeBaseIds.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.KnowledgeBaseIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.KnowledgeBaseIds = &ids
	}
	if !data.ConnectorIds.IsNull() && !data.ConnectorIds.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.ConnectorIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.ConnectorIds = &ids
	}

	// Call API
	var apiResp *client.UpdateAgentResponse
	hasBuiltInConfig := data.BuiltInAgentConfig != nil && !data.BuiltInAgentConfig.Name.IsNull()

	if hasBuiltInConfig {
		// Set builtInAgentConfig in the request
		bodyBytes, marshalErr := json.Marshal(requestBody)
		if marshalErr != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", marshalErr))
			return
		}
		configJSON := r.buildBuiltInAgentConfigJSON(data.BuiltInAgentConfig)
		bodyBytes, marshalErr = r.injectBuiltInAgentConfig(bodyBytes, configJSON)
		if marshalErr != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to inject builtInAgentConfig: %s", marshalErr))
			return
		}
		var updateErr error
		apiResp, updateErr = r.client.UpdateAgentWithBodyWithResponse(ctx, profileID, "application/json", bytes.NewReader(bodyBytes))
		if updateErr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update profile, got error: %s", updateErr))
			return
		}
	} else {
		// Send builtInAgentConfig:null to clear any existing config
		bodyBytes, marshalErr := json.Marshal(requestBody)
		if marshalErr != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", marshalErr))
			return
		}
		bodyBytes, marshalErr = r.injectBuiltInAgentConfig(bodyBytes, json.RawMessage("null"))
		if marshalErr != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to inject null builtInAgentConfig: %s", marshalErr))
			return
		}
		var updateErr error
		apiResp, updateErr = r.client.UpdateAgentWithBodyWithResponse(ctx, profileID, "application/json", bytes.NewReader(bodyBytes))
		if updateErr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update profile, got error: %s", updateErr))
			return
		}
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)
	r.mapResponseFieldsToState(&data, apiResp.JSON200.Description, apiResp.JSON200.Icon, apiResp.JSON200.SystemPrompt, apiResp.JSON200.LlmModel, apiResp.JSON200.LlmApiKeyId, apiResp.JSON200.PassthroughHeaders, apiResp.JSON200.IncomingEmailEnabled, apiResp.JSON200.IncomingEmailAllowedDomain, string(apiResp.JSON200.IncomingEmailSecurityMode), ctx, resp.Diagnostics)

	// Map agent_type from response
	data.AgentType = types.StringValue(string(apiResp.JSON200.AgentType))

	// Map consider_context_untrusted and is_default from response (non-nullable bools)
	data.ConsiderContextUntrusted = types.BoolValue(apiResp.JSON200.ConsiderContextUntrusted)
	data.IsDefault = types.BoolValue(apiResp.JSON200.IsDefault)

	// Map identity_provider_id from response (nullable)
	if apiResp.JSON200.IdentityProviderId != nil {
		data.IdentityProviderId = types.StringValue(*apiResp.JSON200.IdentityProviderId)
	} else if !data.IdentityProviderId.IsNull() {
		data.IdentityProviderId = types.StringNull()
	}

	// Map suggested_prompts from response
	r.mapSuggestedPromptsToState(&data, apiResp.JSON200.SuggestedPrompts)

	// Map knowledge_base_ids from response
	r.mapStringListToState(ctx, &data.KnowledgeBaseIds, apiResp.JSON200.KnowledgeBaseIds, resp.Diagnostics)

	// Map connector_ids from response
	r.mapStringListToState(ctx, &data.ConnectorIds, apiResp.JSON200.ConnectorIds, resp.Diagnostics)

	// Map labels from API response, preserving configuration order
	// If labels were not specified in config (nil), keep them nil in state
	if data.Labels != nil {
		data.Labels = r.mapLabelsToConfigurationOrder(data.Labels, apiResp.JSON200.Labels)
	}

	// Map built_in_agent_config from response
	r.mapBuiltInAgentConfigToState(&data, apiResp.Body)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProfileResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	profileID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.DeleteAgentWithResponse(ctx, profileID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete profile, got error: %s", err))
		return
	}

	// Built-in agents cannot be deleted by the backend (403).
	// Block the destroy and tell the user how to fix it.
	if apiResp.StatusCode() == 403 {
		resp.Diagnostics.AddError(
			"Cannot Delete Built-In Agent",
			"Built-in agents cannot be deleted from Archestra. To delete this profile, first remove the built_in_agent_config block from your configuration and run `terraform apply` to clear it, then run `terraform destroy`.",
		)
		return
	}

	// Check response (200 or 404 are both acceptable for delete)
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *ProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapResponseFieldsToState maps the new optional fields from API response to Terraform state.
func (r *ProfileResource) mapResponseFieldsToState(data *ProfileResourceModel, description *string, icon *string, systemPrompt *string, llmModel *string, llmApiKeyId *openapi_types.UUID, passthroughHeaders *[]string, incomingEmailEnabled bool, incomingEmailAllowedDomain *string, incomingEmailSecurityMode string, ctx context.Context, diags diag.Diagnostics) {
	if description != nil {
		data.Description = types.StringValue(*description)
	} else if !data.Description.IsNull() {
		data.Description = types.StringNull()
	}

	if icon != nil {
		data.Icon = types.StringValue(*icon)
	} else if !data.Icon.IsNull() {
		data.Icon = types.StringNull()
	}

	if systemPrompt != nil {
		data.SystemPrompt = types.StringValue(*systemPrompt)
	} else if !data.SystemPrompt.IsNull() {
		data.SystemPrompt = types.StringNull()
	}

	if llmModel != nil {
		data.LlmModel = types.StringValue(*llmModel)
	} else if !data.LlmModel.IsNull() {
		data.LlmModel = types.StringNull()
	}

	if llmApiKeyId != nil {
		data.LlmApiKeyId = types.StringValue(llmApiKeyId.String())
	} else if !data.LlmApiKeyId.IsNull() {
		data.LlmApiKeyId = types.StringNull()
	}

	if passthroughHeaders != nil {
		headerList, d := types.ListValueFrom(ctx, types.StringType, *passthroughHeaders)
		diags.Append(d...)
		data.PassthroughHeaders = headerList
	} else if !data.PassthroughHeaders.IsNull() {
		data.PassthroughHeaders = types.ListNull(types.StringType)
	}

	data.IncomingEmailEnabled = types.BoolValue(incomingEmailEnabled)

	if incomingEmailAllowedDomain != nil {
		data.IncomingEmailAllowedDomain = types.StringValue(*incomingEmailAllowedDomain)
	} else if !data.IncomingEmailAllowedDomain.IsNull() {
		data.IncomingEmailAllowedDomain = types.StringNull()
	}

	if incomingEmailSecurityMode != "" {
		data.IncomingEmailSecurityMode = types.StringValue(incomingEmailSecurityMode)
	} else if !data.IncomingEmailSecurityMode.IsNull() {
		data.IncomingEmailSecurityMode = types.StringNull()
	}
}

// mapLabelsToConfigurationOrder maps API response labels back to the configuration order
// to ensure Terraform doesn't detect false changes due to API reordering.
func (r *ProfileResource) mapLabelsToConfigurationOrder(configLabels []ProfileLabelModel, apiLabels []struct {
	Key     string              `json:"key"`
	KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
	Value   string              `json:"value"`
	ValueId *openapi_types.UUID `json:"valueId,omitempty"`
}) []ProfileLabelModel {
	// Create a map of API labels for quick lookup
	apiLabelMap := make(map[string]string)
	for _, label := range apiLabels {
		apiLabelMap[label.Key] = label.Value
	}

	// Build result preserving configuration order
	result := make([]ProfileLabelModel, len(configLabels))
	for i, configLabel := range configLabels {
		key := configLabel.Key.ValueString()
		if apiValue, exists := apiLabelMap[key]; exists {
			result[i] = ProfileLabelModel{
				Key:   types.StringValue(key),
				Value: types.StringValue(apiValue),
			}
		} else {
			// Keep original if API doesn't have it (shouldn't happen normally)
			result[i] = configLabel
		}
	}

	return result
}

// mapSuggestedPromptsToState maps suggested prompts from API response to Terraform state.
func (r *ProfileResource) mapSuggestedPromptsToState(data *ProfileResourceModel, apiPrompts []struct {
	Prompt       string `json:"prompt"`
	SummaryTitle string `json:"summaryTitle"`
}) {
	if len(apiPrompts) > 0 {
		prompts := make([]SuggestedPromptModel, len(apiPrompts))
		for i, sp := range apiPrompts {
			prompts[i] = SuggestedPromptModel{
				Prompt:       types.StringValue(sp.Prompt),
				SummaryTitle: types.StringValue(sp.SummaryTitle),
			}
		}
		data.SuggestedPrompts = prompts
	} else if data.SuggestedPrompts != nil {
		data.SuggestedPrompts = nil
	}
}

// mapStringListToState maps a string slice from API response to a types.List in Terraform state.
func (r *ProfileResource) mapStringListToState(ctx context.Context, target *types.List, apiValues []string, diags diag.Diagnostics) {
	if len(apiValues) > 0 {
		list, d := types.ListValueFrom(ctx, types.StringType, apiValues)
		diags.Append(d...)
		*target = list
	} else if !target.IsNull() {
		*target = types.ListNull(types.StringType)
	}
}

// buildBuiltInAgentConfigJSON builds the JSON representation of the built-in agent config
// based on the discriminator name field.
func (r *ProfileResource) buildBuiltInAgentConfigJSON(config *BuiltInAgentConfigModel) json.RawMessage {
	name := config.Name.ValueString()
	switch name {
	case "policy-configuration-subagent":
		autoConfig := config.AutoConfigureOnToolDiscovery.ValueBool()
		b, _ := json.Marshal(map[string]interface{}{
			"name":                         name,
			"autoConfigureOnToolDiscovery": autoConfig,
		})
		return b
	case "dual-llm-main-agent":
		maxRounds := config.MaxRounds.ValueInt64()
		b, _ := json.Marshal(map[string]interface{}{
			"name":      name,
			"maxRounds": maxRounds,
		})
		return b
	case "dual-llm-quarantine-agent":
		b, _ := json.Marshal(map[string]interface{}{
			"name": name,
		})
		return b
	default:
		b, _ := json.Marshal(map[string]interface{}{
			"name": name,
		})
		return b
	}
}

// injectBuiltInAgentConfig injects the builtInAgentConfig field into a JSON-encoded request body.
func (r *ProfileResource) injectBuiltInAgentConfig(body []byte, configJSON json.RawMessage) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	raw["builtInAgentConfig"] = configJSON
	return json.Marshal(raw)
}

// mapBuiltInAgentConfigToState parses the builtInAgentConfig from raw API response body
// and maps it into the Terraform state model.
func (r *ProfileResource) mapBuiltInAgentConfigToState(data *ProfileResourceModel, responseBody []byte) {
	var rawResp struct {
		BuiltInAgentConfig *json.RawMessage `json:"builtInAgentConfig"`
	}
	if err := json.Unmarshal(responseBody, &rawResp); err != nil || rawResp.BuiltInAgentConfig == nil {
		return
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(*rawResp.BuiltInAgentConfig, &configMap); err != nil {
		return
	}

	name, _ := configMap["name"].(string)
	if name == "" {
		return
	}

	result := &BuiltInAgentConfigModel{
		Name:                         types.StringValue(name),
		AutoConfigureOnToolDiscovery: types.BoolNull(),
		MaxRounds:                    types.Int64Null(),
	}

	switch name {
	case "policy-configuration-subagent":
		if v, ok := configMap["autoConfigureOnToolDiscovery"].(bool); ok {
			result.AutoConfigureOnToolDiscovery = types.BoolValue(v)
		}
	case "dual-llm-main-agent":
		if v, ok := configMap["maxRounds"].(float64); ok {
			result.MaxRounds = types.Int64Value(int64(v))
		}
	}

	data.BuiltInAgentConfig = result
}
