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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
				Default:             stringdefault.StaticString("org"),
				MarkdownDescription: "Ownership scope: `personal`, `team`, or `org` (default: `org`).",
				Validators:          []validator.String{stringvalidator.OneOf("personal", "team", "org")},
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
//   - suggestedPrompts/passthroughHeaders: llm_proxy / mcp_gateway-only
//     wire fields; not present on the agent variant of the schema.
//   - identityProviderId: gateway/proxy-only on the schema side (an agent
//     never has one); the wire returns null for agent rows.
//   - slug: auto-generated URL slug, not user-configurable.
//   - tools: list of tool assignments managed by archestra_agent_tool —
//     duplicating it here would create a phantom diff against the m2m
//     relationship.
func (r *AgentResource) KnownIntentionallySkipped() []string {
	return []string{
		"agentType", "authorId", "authorName", "builtIn", "organizationId",
		"createdAt", "updatedAt", "suggestedPrompts", "passthroughHeaders",
		"identityProviderId", "slug", "tools",
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
var agentAttrSpec = []AttrSpec{
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
	{TFName: "consider_context_untrusted", JSONName: "considerContextUntrusted", Kind: Scalar},
	{TFName: "is_default", JSONName: "isDefault", Kind: Scalar},
	{TFName: "scope", JSONName: "scope", Kind: Scalar},
	{TFName: "teams", JSONName: "teams", Kind: List},
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
	{
		TFName: "built_in_agent_config", JSONName: "builtInAgentConfig", Kind: AtomicObject,
		Children: []AttrSpec{
			{TFName: "name", JSONName: "name", Kind: Scalar},
			{TFName: "auto_configure_on_tool_discovery", JSONName: "autoConfigureOnToolDiscovery", Kind: Scalar},
			{TFName: "max_rounds", JSONName: "maxRounds", Kind: Scalar},
		},
		Encoder: encodeBuiltInAgentConfig,
	},
}

// encodeBuiltInAgentConfig narrows the AtomicObject's encoded map to only the
// sub-fields valid for the discriminator's `name` value. Backend zod is a
// discriminated union; sending extra sub-fields would reject.
func encodeBuiltInAgentConfig(v any) any {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return v
	}
	name, _ := m["name"].(string)
	switch name {
	case "policy-configuration-subagent":
		out := map[string]any{"name": name}
		if v, ok := m["autoConfigureOnToolDiscovery"]; ok {
			out["autoConfigureOnToolDiscovery"] = v
		}
		return out
	case "dual-llm-main-agent":
		out := map[string]any{"name": name}
		if v, ok := m["maxRounds"]; ok {
			out["maxRounds"] = v
		}
		return out
	case "dual-llm-quarantine-agent":
		return map[string]any{"name": name}
	}
	return m
}
