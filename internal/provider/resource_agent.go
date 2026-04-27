package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &AgentResource{}
var _ resource.ResourceWithImportState = &AgentResource{}

func NewAgentResource() resource.Resource { return &AgentResource{} }

type AgentResource struct {
	client *client.ClientWithResponses
}

// AgentResourceModel is the schema for an internal Archestra agent (chat agent
// with prompts, knowledge sources, optional email triggers, etc.).
type AgentResourceModel struct {
	ID                         types.String             `tfsdk:"id"`
	Name                       types.String             `tfsdk:"name"`
	Description                types.String             `tfsdk:"description"`
	Icon                       types.String             `tfsdk:"icon"`
	SystemPrompt               types.String             `tfsdk:"system_prompt"`
	LlmModel                   types.String             `tfsdk:"llm_model"`
	LlmApiKeyId                types.String             `tfsdk:"llm_api_key_id"`
	KnowledgeBaseIds           types.List               `tfsdk:"knowledge_base_ids"`
	ConnectorIds               types.List               `tfsdk:"connector_ids"`
	IncomingEmailEnabled       types.Bool               `tfsdk:"incoming_email_enabled"`
	IncomingEmailAllowedDomain types.String             `tfsdk:"incoming_email_allowed_domain"`
	IncomingEmailSecurityMode  types.String             `tfsdk:"incoming_email_security_mode"`
	ConsiderContextUntrusted   types.Bool               `tfsdk:"consider_context_untrusted"`
	IsDefault                  types.Bool               `tfsdk:"is_default"`
	SuggestedPrompts           []SuggestedPromptModel   `tfsdk:"suggested_prompts"`
	Labels                     []AgentLabelModel        `tfsdk:"labels"`
	BuiltInAgentConfig         *BuiltInAgentConfigModel `tfsdk:"built_in_agent_config"`
	Scope                      types.String             `tfsdk:"scope"`
	Teams                      types.List               `tfsdk:"teams"`
}

func (r *AgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (r *AgentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra internal agent — a chat agent backed by an LLM, optionally augmented by knowledge bases, connectors, and email triggers.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Agent identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Agent name",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Human-readable description",
			},
			"icon": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Emoji or base64 image data URL",
			},
			"system_prompt": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "System prompt that frames the agent's behavior",
			},
			"llm_model": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Model ID used for LLM calls",
			},
			"llm_api_key_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ID of the LLM provider API key the agent should use",
			},
			"knowledge_base_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Knowledge base IDs the agent has access to",
			},
			"connector_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Knowledge connector IDs the agent has access to",
			},
			"incoming_email_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether incoming-email invocation is enabled",
			},
			"incoming_email_allowed_domain": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Allowed sender domain when `incoming_email_security_mode = \"internal\"`",
			},
			"incoming_email_security_mode": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Email-trigger security mode. One of `private` (only the agent owner can email it), `internal` (any sender from `incoming_email_allowed_domain` can), or `public` (any sender). Defaults to `private`.",
				Validators: []validator.String{
					stringvalidator.OneOf("private", "internal", "public"),
				},
			},
			"consider_context_untrusted": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the agent context is treated as untrusted",
			},
			"is_default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this is the default agent for its type",
			},
			"scope": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Ownership scope: `personal`, `team`, or `org` (default: `org`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"teams": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Team IDs this agent is assigned to. Required when `scope = \"team\"`.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"suggested_prompts": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Suggested prompts surfaced to users in the chat UI",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"prompt":        schema.StringAttribute{Required: true, MarkdownDescription: "Prompt text"},
						"summary_title": schema.StringAttribute{Required: true, MarkdownDescription: "Title shown above the prompt"},
					},
				},
			},
			"labels": schema.SetNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Key/value labels for organizing agents",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":   schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
		},
		Blocks: map[string]schema.Block{
			"built_in_agent_config": schema.SingleNestedBlock{
				MarkdownDescription: "Built-in agent configuration. Discriminated by `name`.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Built-in agent identifier: `policy-configuration-subagent`, `dual-llm-main-agent`, `dual-llm-quarantine-agent`",
					},
					"auto_configure_on_tool_discovery": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Only applies when `name = \"policy-configuration-subagent\"`",
					},
					"max_rounds": schema.Int64Attribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Only applies when `name = \"dual-llm-main-agent\"` (1–20)",
						PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
					},
				},
			},
		},
	}
}

func (r *AgentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *AgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scope := client.CreateAgentJSONBodyScopeOrg
	if !data.Scope.IsNull() && !data.Scope.IsUnknown() {
		scope = client.CreateAgentJSONBodyScope(data.Scope.ValueString())
	}
	teams := []string{}
	if !data.Teams.IsNull() && !data.Teams.IsUnknown() {
		resp.Diagnostics.Append(data.Teams.ElementsAs(ctx, &teams, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	labels := buildAgentLabelsCreate(data.Labels)
	requestBody := client.CreateAgentJSONRequestBody{
		Name:   data.Name.ValueString(),
		Scope:  scope,
		Teams:  &teams,
		Labels: &labels,
	}

	agentType := client.CreateAgentJSONBodyAgentType("agent")
	requestBody.AgentType = &agentType

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		requestBody.Description = &v
	}
	if !data.Icon.IsNull() && !data.Icon.IsUnknown() {
		v := data.Icon.ValueString()
		requestBody.Icon = &v
	}
	if !data.SystemPrompt.IsNull() && !data.SystemPrompt.IsUnknown() {
		v := data.SystemPrompt.ValueString()
		requestBody.SystemPrompt = &v
	}
	if !data.LlmModel.IsNull() && !data.LlmModel.IsUnknown() {
		v := data.LlmModel.ValueString()
		requestBody.LlmModel = &v
	}
	if !data.LlmApiKeyId.IsNull() && !data.LlmApiKeyId.IsUnknown() {
		id, parseErr := uuid.Parse(data.LlmApiKeyId.ValueString())
		if parseErr != nil {
			resp.Diagnostics.AddError("Invalid llm_api_key_id", fmt.Sprintf("Unable to parse llm_api_key_id: %s", parseErr))
			return
		}
		requestBody.LlmApiKeyId = &id
	}
	if !data.IncomingEmailEnabled.IsNull() && !data.IncomingEmailEnabled.IsUnknown() {
		v := data.IncomingEmailEnabled.ValueBool()
		requestBody.IncomingEmailEnabled = &v
	}
	if !data.IncomingEmailAllowedDomain.IsNull() && !data.IncomingEmailAllowedDomain.IsUnknown() {
		v := data.IncomingEmailAllowedDomain.ValueString()
		requestBody.IncomingEmailAllowedDomain = &v
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
	if data.SuggestedPrompts != nil {
		prompts := suggestedPromptsToAPI(data.SuggestedPrompts)
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

	var apiResp *client.CreateAgentResponse
	if data.BuiltInAgentConfig != nil && !data.BuiltInAgentConfig.Name.IsNull() {
		bodyBytes, marshalErr := json.Marshal(requestBody)
		if marshalErr != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", marshalErr))
			return
		}
		bodyBytes, marshalErr = injectBuiltInAgentConfig(bodyBytes, buildBuiltInAgentConfigJSON(data.BuiltInAgentConfig))
		if marshalErr != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to inject builtInAgentConfig: %s", marshalErr))
			return
		}
		var createErr error
		apiResp, createErr = r.client.CreateAgentWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
		if createErr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create agent, got error: %s", createErr))
			return
		}
	} else {
		var createErr error
		apiResp, createErr = r.client.CreateAgentWithResponse(ctx, requestBody)
		if createErr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create agent, got error: %s", createErr))
			return
		}
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	r.flattenAgentResponse(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.GetAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent, got error: %s", err))
		return
	}
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	r.flattenAgentResponse(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	labels := buildAgentLabelsUpdate(data.Labels)
	name := data.Name.ValueString()
	requestBody := client.UpdateAgentJSONRequestBody{
		Name:   &name,
		Labels: &labels,
	}
	at := client.UpdateAgentJSONBodyAgentType("agent")
	requestBody.AgentType = &at

	if !data.Scope.IsNull() && !data.Scope.IsUnknown() {
		s := client.UpdateAgentJSONBodyScope(data.Scope.ValueString())
		requestBody.Scope = &s
	}
	if !data.Teams.IsNull() && !data.Teams.IsUnknown() {
		var teams []string
		resp.Diagnostics.Append(data.Teams.ElementsAs(ctx, &teams, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.Teams = &teams
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		requestBody.Description = &v
	}
	if !data.Icon.IsNull() && !data.Icon.IsUnknown() {
		v := data.Icon.ValueString()
		requestBody.Icon = &v
	}
	if !data.SystemPrompt.IsNull() && !data.SystemPrompt.IsUnknown() {
		v := data.SystemPrompt.ValueString()
		requestBody.SystemPrompt = &v
	}
	if !data.LlmModel.IsNull() && !data.LlmModel.IsUnknown() {
		v := data.LlmModel.ValueString()
		requestBody.LlmModel = &v
	}
	if !data.LlmApiKeyId.IsNull() && !data.LlmApiKeyId.IsUnknown() {
		uid, parseErr := uuid.Parse(data.LlmApiKeyId.ValueString())
		if parseErr != nil {
			resp.Diagnostics.AddError("Invalid llm_api_key_id", fmt.Sprintf("Unable to parse llm_api_key_id: %s", parseErr))
			return
		}
		requestBody.LlmApiKeyId = &uid
	}
	if !data.IncomingEmailEnabled.IsNull() && !data.IncomingEmailEnabled.IsUnknown() {
		v := data.IncomingEmailEnabled.ValueBool()
		requestBody.IncomingEmailEnabled = &v
	}
	if !data.IncomingEmailAllowedDomain.IsNull() && !data.IncomingEmailAllowedDomain.IsUnknown() {
		v := data.IncomingEmailAllowedDomain.ValueString()
		requestBody.IncomingEmailAllowedDomain = &v
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
	if data.SuggestedPrompts != nil {
		prompts := suggestedPromptsToAPI(data.SuggestedPrompts)
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

	bodyBytes, marshalErr := json.Marshal(requestBody)
	if marshalErr != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", marshalErr))
		return
	}
	if data.BuiltInAgentConfig != nil && !data.BuiltInAgentConfig.Name.IsNull() {
		bodyBytes, marshalErr = injectBuiltInAgentConfig(bodyBytes, buildBuiltInAgentConfigJSON(data.BuiltInAgentConfig))
	} else {
		bodyBytes, marshalErr = injectBuiltInAgentConfig(bodyBytes, json.RawMessage("null"))
	}
	if marshalErr != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to inject builtInAgentConfig: %s", marshalErr))
		return
	}

	apiResp, updateErr := r.client.UpdateAgentWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(bodyBytes))
	if updateErr != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update agent, got error: %s", updateErr))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	r.flattenAgentResponse(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete agent, got error: %s", err))
		return
	}
	if apiResp.StatusCode() == 403 {
		resp.Diagnostics.AddError(
			"Cannot Delete Built-In Agent",
			"Built-in agents cannot be deleted from Archestra. Remove the `built_in_agent_config` block and apply, then run `terraform destroy`.",
		)
		return
	}
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
	}
}

func (r *AgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// flattenAgentResponse maps an agent API response (decoded from the raw body
// bytes) into the AgentResourceModel state.
func (r *AgentResource) flattenAgentResponse(ctx context.Context, data *AgentResourceModel, body []byte, diags *diag.Diagnostics) {
	resp := parseAgentResponse(body, diags)
	if resp == nil {
		return
	}

	data.ID = types.StringValue(resp.Id.String())
	data.Name = types.StringValue(resp.Name)
	optionalStringFromAPI(&data.Description, resp.Description)
	optionalStringFromAPI(&data.Icon, resp.Icon)
	optionalStringFromAPI(&data.SystemPrompt, resp.SystemPrompt)
	optionalStringFromAPI(&data.LlmModel, resp.LlmModel)
	optionalUUIDFromAPI(&data.LlmApiKeyId, resp.LlmApiKeyId)

	data.IncomingEmailEnabled = types.BoolValue(resp.IncomingEmailEnabled)
	optionalStringFromAPI(&data.IncomingEmailAllowedDomain, resp.IncomingEmailAllowedDomain)
	if resp.IncomingEmailSecurityMode != "" {
		data.IncomingEmailSecurityMode = types.StringValue(resp.IncomingEmailSecurityMode)
	} else if !data.IncomingEmailSecurityMode.IsNull() {
		data.IncomingEmailSecurityMode = types.StringNull()
	}

	data.ConsiderContextUntrusted = types.BoolValue(resp.ConsiderContextUntrusted)
	data.IsDefault = types.BoolValue(resp.IsDefault)
	data.Scope = types.StringValue(resp.Scope)
	data.Teams = teamsListFromAPI(ctx, resp.Teams, diags)

	data.SuggestedPrompts = suggestedPromptsFromAPI(resp.SuggestedPrompts)
	stringListFromAPI(ctx, &data.KnowledgeBaseIds, resp.KnowledgeBaseIds, diags)
	stringListFromAPI(ctx, &data.ConnectorIds, resp.ConnectorIds, diags)

	data.Labels = flattenAgentLabels(data.Labels, resp.Labels)

	data.BuiltInAgentConfig = builtInAgentConfigFromResponse(body)
}
