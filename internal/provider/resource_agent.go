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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ resource.Resource                   = &AgentResource{}
	_ resource.ResourceWithImportState    = &AgentResource{}
	_ resource.ResourceWithValidateConfig = &AgentResource{}
)

func NewAgentResource() resource.Resource { return &AgentResource{} }

type AgentResource struct {
	client *client.ClientWithResponses
}

// AgentResourceModel is the schema for an internal Archestra agent (chat agent
// with prompts, knowledge sources, optional email triggers, etc.).
type AgentResourceModel struct {
	Tools                          types.Set    `tfsdk:"tools"`
	ExcludedToolIDs                types.Set    `tfsdk:"excluded_tool_ids"`
	Hooks                          types.Set    `tfsdk:"hooks"`
	SubagentIDs                    types.Set    `tfsdk:"subagent_ids"`
	RemoteAgentIDs                 types.Set    `tfsdk:"remote_agent_ids"`
	ExcludedSubagentIDs            types.Set    `tfsdk:"excluded_subagent_ids"`
	ExcludedConnectorIDs           types.Set    `tfsdk:"excluded_connector_ids"`
	SkillPolicy                    types.Object `tfsdk:"skills"`
	SkillPolicyHiddenAllowedCount  types.Int64  `tfsdk:"skill_policy_hidden_allowed_count"`
	SkillPolicyHiddenExcludedCount types.Int64  `tfsdk:"skill_policy_hidden_excluded_count"`

	InitialGrants                        types.Set    `tfsdk:"permissions"`
	AuthorEmail                          types.String `tfsdk:"author_email"`
	CreatedBy                            types.String `tfsdk:"created_by"`
	CreatedByServiceAccountId            types.String `tfsdk:"created_by_service_account_id"`
	DeletedAt                            types.String `tfsdk:"deleted_at"`
	LastUsedAt                           types.String `tfsdk:"last_used_at"`
	LatestVersion                        types.Int64  `tfsdk:"latest_version"`
	ActivationSkillMode                  types.String `tfsdk:"activation_skill_mode"`
	ActivationSkillPolicyRevision        types.Int64  `tfsdk:"activation_skill_policy_revision"`
	ActivationSkillsCount                types.Int64  `tfsdk:"activation_skills_count"`
	LlmProviderRequiresPerUserCredential types.Bool   `tfsdk:"llm_provider_requires_per_user_credential"`
	ResolvedLlmModelName                 types.String `tfsdk:"resolved_llm_model_name"`
	ResolvedLlmProvider                  types.String `tfsdk:"resolved_llm_provider"`
	ResolvedLlmProviderKeyName           types.String `tfsdk:"resolved_llm_provider_key_name"`
	SandboxAvailable                     types.Bool   `tfsdk:"sandbox_available"`
	AccessAllSubagents                   types.Bool   `tfsdk:"access_all_subagents"`
	ToolExposureMode                     types.String `tfsdk:"tool_exposure_mode"`

	ID                         types.String           `tfsdk:"id"`
	Name                       types.String           `tfsdk:"name"`
	Description                types.String           `tfsdk:"description"`
	Icon                       types.String           `tfsdk:"icon"`
	SystemPrompt               types.String           `tfsdk:"system_prompt"`
	LlmModel                   types.String           `tfsdk:"llm_model"`
	LlmApiKeyId                types.String           `tfsdk:"llm_api_key_id"`
	KnowledgeBaseIds           types.List             `tfsdk:"knowledge_base_ids"`
	ConnectorIds               types.List             `tfsdk:"connector_ids"`
	IncomingEmailEnabled       types.Bool             `tfsdk:"incoming_email_enabled"`
	IncomingEmailAllowedDomain types.String           `tfsdk:"incoming_email_allowed_domain"`
	IncomingEmailSecurityMode  types.String           `tfsdk:"incoming_email_security_mode"`
	IsDefault                  types.Bool             `tfsdk:"is_default"`
	SuggestedPrompts           []SuggestedPromptModel `tfsdk:"suggested_prompts"`
	Labels                     []AgentLabelModel      `tfsdk:"labels"`
	BuiltInAgentConfig         types.String           `tfsdk:"built_in_agent_config"`

	AccessAllTools            types.Bool         `tfsdk:"access_all_tools"`
	MissingCredentialBehavior types.String       `tfsdk:"missing_credential_behavior"`
	ModelId                   types.String       `tfsdk:"model_id"`
	EnvironmentId             types.String       `tfsdk:"environment_id"`
	Runtime                   *AgentRuntimeModel `tfsdk:"runtime"`
}

func (r *AgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (r *AgentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:             1,
		MarkdownDescription: "Internal chat agent — system prompt + LLM, optionally augmented with knowledge bases, connectors, and email triggers.",
		Attributes: map[string]schema.Attribute{
			"permissions": initialGrantsAttribute(),
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
			"is_default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this is the default agent for its type",
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
			"access_all_tools": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Auto-tool mode: the agent sees every tool it can reach instead of only pinned assignments. Use excluded_tool_ids to exclude tools. Defaults to `false`.",
			},
			"missing_credential_behavior": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "What happens when a caller lacks a connection the agent's tools need: `allow` (fail only at invocation, default), `warn` (surface missing connections up front), or `block` (agent unusable until connected)",
				Validators: []validator.String{
					stringvalidator.OneOf("allow", "warn", "block"),
				},
			},
			"model_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Database UUID of the pinned LLM model row (`dbId` from `/api/llm-models/available`) — not the provider-facing model name, which is `llm_model`",
			},
			"environment_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Environment UUID scoping which MCP servers and connections the agent resolves against; omit for the default environment",
			},
			"runtime":               runtimeSchema(),
			"built_in_agent_config": schema.StringAttribute{Computed: true, MarkdownDescription: "Server-owned built-in agent metadata as JSON."},
		},
	}
	resp.Schema.Attributes["hooks"] = agentHooksAttribute()
	resp.Schema.Attributes["tools"] = agentToolsAttribute()
	resp.Schema.Attributes["excluded_tool_ids"] = agentIDSetAttribute("Tools excluded from automatic tool access; omit to preserve or set an empty set to clear.", 0)
	for name, attribute := range agentRelationshipAttributes() {
		resp.Schema.Attributes[name] = attribute
	}
	for name, attribute := range currentAgentAttributes() {
		resp.Schema.Attributes[name] = attribute
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
	defer finishParentCreate(ctx, r.client, "agent", "id", resp)

	// Run merge-patch with prior=null Object: every non-null plan attribute
	// emits, which is exactly Create semantics.
	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, agentAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// agent_type isn't surfaced as a TF attribute (the resource is type-specific
	// by construction); set it on the wire so the backend stores the right
	// discriminator.
	patch["agentType"] = "agent"

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal merge patch: %s", err))
		return
	}
	LogPatch(ctx, "archestra_agent Create", patch, agentAttrSpec)

	apiResp, err := r.client.CreateAgentWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create agent: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	var data AgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.flattenAgentResponse(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	partial := data
	nullUnknownAgentRelationships(&partial)
	resp.Diagnostics.Append(resp.State.Set(ctx, &partial)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.writeRelationships(ctx, &data, &AgentResourceModel{}, req.Config, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		r.writeHooks(ctx, &data, &AgentResourceModel{}, req.Config, &resp.Diagnostics)
		if !resp.Diagnostics.HasError() {
			r.writeTools(ctx, &data, &AgentResourceModel{}, req.Config, &resp.Diagnostics)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	r.refreshMetadataAfterWrite(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readRelationships(ctx, &data, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		r.readHooks(ctx, &data, &resp.Diagnostics)
		if !resp.Diagnostics.HasError() {
			r.readTools(ctx, &data, &resp.Diagnostics)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
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
	r.readRelationships(ctx, &data, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		r.readHooks(ctx, &data, &resp.Diagnostics)
		if !resp.Diagnostics.HasError() {
			r.readTools(ctx, &data, &resp.Diagnostics)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resolveInitialGrants(&data.InitialGrants)
	readParentPermissions(ctx, r.client, "agent", data.ID.ValueString(), &data.InitialGrants, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	finishPermissions := beginParentPermissionUpdate(ctx, r.client, "agent", "id", req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	defer finishPermissions()

	var stateData AgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(stateData.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, agentAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_agent Update", patch, agentAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal merge patch: %s", err))
		return
	}

	apiResp, err := r.client.UpdateAgentWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update agent: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Resource Deleted Outside Terraform",
			"The resource was deleted on the backend between refresh and apply. "+
				"Re-run `terraform apply` — the next refresh drops it from state and the plan recreates it.",
		)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	var data AgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.flattenAgentResponse(ctx, &data, apiResp.Body, &resp.Diagnostics)
	r.writeRelationships(ctx, &data, &stateData, req.Config, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		r.writeHooks(ctx, &data, &stateData, req.Config, &resp.Diagnostics)
		if !resp.Diagnostics.HasError() {
			r.writeTools(ctx, &data, &stateData, req.Config, &resp.Diagnostics)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	r.refreshMetadataAfterWrite(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readRelationships(ctx, &data, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		r.readHooks(ctx, &data, &resp.Diagnostics)
		if !resp.Diagnostics.HasError() {
			r.readTools(ctx, &data, &resp.Diagnostics)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resolveInitialGrants(&data.InitialGrants)
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

// ValidateConfig checks dependent configuration and skill reference shapes.
func (r *AgentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateInlineSkillPolicy(ctx, req.Config, &resp.Diagnostics)
	validateInlineHooks(ctx, req.Config, &resp.Diagnostics)
	validateInlineTools(ctx, req.Config, &resp.Diagnostics)
	var data AgentResourceModel
	// Decode only the fields this validator uses. Runtime objects can be
	// unknown during validation (for example, a conditional inside for_each).
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("incoming_email_security_mode"), &data.IncomingEmailSecurityMode)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("incoming_email_allowed_domain"), &data.IncomingEmailAllowedDomain)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.IncomingEmailSecurityMode.IsNull() && !data.IncomingEmailSecurityMode.IsUnknown() &&
		data.IncomingEmailSecurityMode.ValueString() == "internal" {
		if data.IncomingEmailAllowedDomain.IsNull() || data.IncomingEmailAllowedDomain.IsUnknown() ||
			data.IncomingEmailAllowedDomain.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(
				path.Root("incoming_email_allowed_domain"),
				"Missing Required Attribute",
				`incoming_email_allowed_domain must be set when incoming_email_security_mode = "internal"`,
			)
		}
	}
}

// flattenAgentResponse maps an agent API response (decoded from the raw body
// bytes) into the AgentResourceModel state.
func (r *AgentResource) flattenAgentResponse(ctx context.Context, data *AgentResourceModel, body []byte, diags *diag.Diagnostics) {
	resp := parseAgentResponse(body, diags)
	if resp == nil {
		return
	}

	flattenCurrentAgentFields(data, body, diags)
	if diags.HasError() {
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

	data.IsDefault = types.BoolValue(resp.IsDefault)

	data.SuggestedPrompts = suggestedPromptsFromAPI(resp.SuggestedPrompts)
	stringListFromAPI(ctx, &data.KnowledgeBaseIds, resp.KnowledgeBaseIds, diags)
	stringListFromAPI(ctx, &data.ConnectorIds, resp.ConnectorIds, diags)

	data.Labels = flattenAgentLabels(data.Labels, resp.Labels)

	data.BuiltInAgentConfig = types.StringNull()
	if resp.BuiltInAgentConfig != nil {
		data.BuiltInAgentConfig = types.StringValue(string(*resp.BuiltInAgentConfig))
	}

	data.AccessAllTools = types.BoolValue(resp.AccessAllTools)
	if resp.MissingCredentialBehavior != "" {
		data.MissingCredentialBehavior = types.StringValue(resp.MissingCredentialBehavior)
	} else if !data.MissingCredentialBehavior.IsNull() {
		data.MissingCredentialBehavior = types.StringNull()
	}
	optionalUUIDFromAPI(&data.ModelId, resp.ModelId)
	optionalStringFromAPI(&data.EnvironmentId, resp.EnvironmentId)
	data.Runtime = runtimeFromResponse(ctx, body, diags)
}

// AttrSpecs implements the resourceWithAttrSpec interface (see specdrift_test.go).
// Activates the schema ↔ AttrSpec drift lint for this resource.
func (r *AgentResource) AttrSpecs() []AttrSpec { return agentAttrSpec }

func (r *AgentResource) APIShape() any { return client.GetAgentResponse{} }

// KnownIntentionallySkipped — wire fields the provider deliberately doesn't
// model on this resource:
//   - agentType: discriminator the provider uses to split one backend table
//     into three resources; not user-facing.
//   - authorId/authorName/builtIn/organizationId/createdAt/updatedAt:
//     audit/ownership metadata; could be added as Computed-only later if
//     users ask, but no consumer has requested it yet.
//   - suggestedPrompts/passthroughHeaders: mcp_gateway-only
//     wire fields; not present on the agent variant of the schema.
//   - identityProviderId: gateway/proxy-only on the schema side (an agent
//     never has one); the wire returns null for agent rows.
//   - slug: auto-generated URL slug, not user-configurable.
//
// Tool assignments, subagents and skills are managed by inline agent fields.
func (r *AgentResource) KnownIntentionallySkipped() []string {
	// Legacy visibility/skill projections and secret-storage pointers are not managed.
	// Activation policy is represented by its current mode/revision metadata.
	return []string{
		"accessAllSkills", "isPersonalGateway", "isPersonalProxy", "runtimeSecretId",
		"agentType", "authorId", "authorName", "builtIn", "organizationId",
		"createdAt", "updatedAt", "suggestedPrompts", "passthroughHeaders",
		"identityProviderId", "slug", "scope", "teams", "users",
	}
}

// agentAttrSpec declares the wire shape for `archestra_agent`. Kept adjacent
// to Schema() so a contributor changing one notices the other.
//
// Storage notes (per platform/backend/src/database/schemas/agent.ts):
//   - Most fields are top-level columns → Scalar / List / Set.
//   - `passthrough_headers` is `text("passthrough_headers").array()` (Postgres
//     text[]) — atomic on the wire, so List with no element-spec.
//   - `built_in_agent_config` is `jsonb("built_in_agent_config")` → AtomicObject
//     with a polymorphic Encoder (the wire shape depends on the `name`
//     discriminator).
//   - `teams`, `labels`, `knowledge_base_ids`, `connector_ids`,
//     `suggested_prompts` are many-to-many sync'd by AgentModel.update via
//     `if (field !== undefined)` per-field guards. Merge-patch's "omit when
//     equal, emit when changed" lines up with that exactly.
var agentAttrSpec = append(append(agentRelationshipAttrSpecs(), AttrSpec{TFName: "hooks", Kind: Synthetic}, AttrSpec{TFName: "tools", Kind: Synthetic}, AttrSpec{TFName: "excluded_tool_ids", Kind: Synthetic}), []AttrSpec{
	initialGrantsAttrSpec,
	{TFName: "access_all_subagents", JSONName: "accessAllSubagents", Kind: Scalar, OmitOnNull: true},
	{TFName: "tool_exposure_mode", JSONName: "toolExposureMode", Kind: Scalar, OmitOnNull: true},

	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "system_prompt", JSONName: "systemPrompt", Kind: Scalar},
	{TFName: "llm_model", JSONName: "llmModel", Kind: Scalar},
	{TFName: "llm_api_key_id", JSONName: "llmApiKeyId", Kind: Scalar},
	{TFName: "knowledge_base_ids", JSONName: "knowledgeBaseIds", Kind: List},
	{TFName: "connector_ids", JSONName: "connectorIds", Kind: List},
	{TFName: "incoming_email_enabled", JSONName: "incomingEmailEnabled", Kind: Scalar},
	{TFName: "incoming_email_allowed_domain", JSONName: "incomingEmailAllowedDomain", Kind: Scalar},
	{TFName: "incoming_email_security_mode", JSONName: "incomingEmailSecurityMode", Kind: Scalar},
	{TFName: "is_default", JSONName: "isDefault", Kind: Scalar},
	{
		TFName: "suggested_prompts", JSONName: "suggestedPrompts", Kind: List,
		Children: []AttrSpec{
			{TFName: "prompt", JSONName: "prompt", Kind: Scalar},
			{TFName: "summary_title", JSONName: "summaryTitle", Kind: Scalar},
		},
	},
	{
		TFName: "labels", JSONName: "labels", Kind: Set,
		Children: []AttrSpec{
			{TFName: "key", JSONName: "key", Kind: Scalar},
			{TFName: "value", JSONName: "value", Kind: Scalar},
		},
	},
	// accessAllTools / missingCredentialBehavior are non-nullable optionals on
	// the backend zod (columns with defaults) — null would 400, so removal
	// means "stop managing". modelId / environmentId are nullable columns and
	// accept an explicit null to clear.
	{TFName: "access_all_tools", JSONName: "accessAllTools", Kind: Scalar, OmitOnNull: true},
	{TFName: "missing_credential_behavior", JSONName: "missingCredentialBehavior", Kind: Scalar, OmitOnNull: true},
	{TFName: "model_id", JSONName: "modelId", Kind: Scalar},
	{TFName: "environment_id", JSONName: "environmentId", Kind: Scalar},
	{
		TFName: "runtime", JSONName: "runtime", Kind: AtomicObject,
		Children: runtimeAttrSpec,
		Encoder:  encodeRuntime,
	},
}...)

func (r *AgentResource) refreshMetadataAfterWrite(ctx context.Context, data *AgentResourceModel, diags *diag.Diagnostics) {
	identifier, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Created Identifier", "The API did not return a valid agent ID.")
		return
	}
	response, err := r.client.GetAgentWithResponse(ctx, identifier)
	if err != nil {
		diags.AddError("Agent Refresh Failed", "The object was written but its full metadata could not be read. Refresh state before retrying.")
		return
	}
	if response.JSON200 == nil {
		diags.AddError("Agent Refresh Failed", fmt.Sprintf("The object was written but metadata read returned %d. Refresh state before retrying.", response.StatusCode()))
		return
	}
	r.flattenAgentResponse(ctx, data, response.Body, diags)
}
