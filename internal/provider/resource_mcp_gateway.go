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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &McpGatewayResource{}
var _ resource.ResourceWithImportState = &McpGatewayResource{}

func NewMcpGatewayResource() resource.Resource { return &McpGatewayResource{} }

type McpGatewayResource struct {
	client *client.ClientWithResponses
}

// McpGatewayResourceModel is the schema for an Archestra MCP gateway. The
// gateway exposes a unified MCP endpoint backed by tool installations and
// optional knowledge sources.
type McpGatewayResourceModel struct {
	ID                       types.String      `tfsdk:"id"`
	Name                     types.String      `tfsdk:"name"`
	Description              types.String      `tfsdk:"description"`
	Icon                     types.String      `tfsdk:"icon"`
	KnowledgeBaseIds         types.List        `tfsdk:"knowledge_base_ids"`
	ConnectorIds             types.List        `tfsdk:"connector_ids"`
	PassthroughHeaders       types.List        `tfsdk:"passthrough_headers"`
	IdentityProviderId       types.String      `tfsdk:"identity_provider_id"`
	ConsiderContextUntrusted types.Bool        `tfsdk:"consider_context_untrusted"`
	IsDefault                types.Bool        `tfsdk:"is_default"`
	Scope                    types.String      `tfsdk:"scope"`
	Teams                    types.List        `tfsdk:"teams"`
	Labels                   []AgentLabelModel `tfsdk:"labels"`
}

func (r *McpGatewayResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_gateway"
}

func (r *McpGatewayResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Unified MCP endpoint that aggregates installed tools and (optionally) knowledge sources, with optional inbound JWT auth via `identity_provider_id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "MCP gateway identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Gateway name"},
			"description": schema.StringAttribute{Optional: true, MarkdownDescription: "Human-readable description"},
			"icon":        schema.StringAttribute{Optional: true, MarkdownDescription: "Emoji or base64 image data URL"},
			"knowledge_base_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Knowledge base IDs the gateway has access to",
			},
			"connector_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Knowledge connector IDs the gateway has access to",
			},
			"passthrough_headers": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Allowlist of HTTP header names to forward from gateway requests to downstream MCP servers",
			},
			"identity_provider_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Identity provider used to validate inbound JWTs. Reference an `archestra_identity_provider`. Omit to disable JWT auth.",
			},
			"consider_context_untrusted": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the gateway context is treated as untrusted",
			},
			"is_default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this is the default MCP gateway",
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
				MarkdownDescription: "Team IDs this gateway is assigned to. Required when `scope = \"team\"`. Removing from configuration clears the assignment on next apply.",
				PlanModifiers:       []planmodifier.List{EmptyListOnConfigNull()},
			},
			"labels": schema.SetNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Key/value labels for organizing gateways",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":   schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
		},
	}
}

func (r *McpGatewayResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *McpGatewayResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, mcpGatewayAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	patch["agentType"] = "mcp_gateway"
	LogPatch(ctx, "archestra_mcp_gateway Create", patch, mcpGatewayAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal merge patch: %s", err))
		return
	}

	apiResp, err := r.client.CreateAgentWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create MCP gateway: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	var data McpGatewayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.flatten(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpGatewayResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpGatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP gateway ID: %s", err))
		return
	}

	apiResp, err := r.client.GetAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read MCP gateway, got error: %s", err))
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

	r.flatten(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpGatewayResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var stateData McpGatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(stateData.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP gateway ID: %s", err))
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, mcpGatewayAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_mcp_gateway Update", patch, mcpGatewayAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal merge patch: %s", err))
		return
	}

	apiResp, err := r.client.UpdateAgentWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update MCP gateway: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	var data McpGatewayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.flatten(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpGatewayResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpGatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP gateway ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete MCP gateway, got error: %s", err))
		return
	}
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
	}
}

func (r *McpGatewayResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *McpGatewayResource) flatten(ctx context.Context, data *McpGatewayResourceModel, body []byte, diags *diag.Diagnostics) {
	resp := parseAgentResponse(body, diags)
	if resp == nil {
		return
	}

	data.ID = types.StringValue(resp.Id.String())
	data.Name = types.StringValue(resp.Name)
	optionalStringFromAPI(&data.Description, resp.Description)
	optionalStringFromAPI(&data.Icon, resp.Icon)
	optionalStringFromAPI(&data.IdentityProviderId, resp.IdentityProviderId)

	stringListFromAPI(ctx, &data.KnowledgeBaseIds, resp.KnowledgeBaseIds, diags)
	stringListFromAPI(ctx, &data.ConnectorIds, resp.ConnectorIds, diags)

	if resp.PassthroughHeaders != nil {
		list, d := types.ListValueFrom(ctx, types.StringType, *resp.PassthroughHeaders)
		diags.Append(d...)
		data.PassthroughHeaders = list
	} else if !data.PassthroughHeaders.IsNull() {
		data.PassthroughHeaders = types.ListNull(types.StringType)
	}

	data.ConsiderContextUntrusted = types.BoolValue(resp.ConsiderContextUntrusted)
	data.IsDefault = types.BoolValue(resp.IsDefault)
	data.Scope = types.StringValue(resp.Scope)
	data.Teams = teamsListFromAPI(ctx, data.Teams, resp.Teams, diags)

	data.Labels = flattenAgentLabels(data.Labels, resp.Labels)
}

// AttrSpecs implements resourceWithAttrSpec — activates the schema↔AttrSpec
// drift lint for this resource.
func (r *McpGatewayResource) AttrSpecs() []AttrSpec { return mcpGatewayAttrSpec }

func (r *McpGatewayResource) APIShape() any { return client.GetAgentResponse{} }

// KnownIntentionallySkipped — wire fields not modeled on archestra_mcp_gateway.
// Same agent-table discriminator + audit-field situation as archestra_llm_proxy.
// Fields excluded here belong to archestra_agent (system_prompt,
// built_in_agent_config, llm config, incoming email, suggested prompts) or
// aren't relevant to a gateway (isDefault, scope, teams,
// considerContextUntrusted). slug + tools follow the same convention as
// the other agent-table resources.
func (r *McpGatewayResource) KnownIntentionallySkipped() []string {
	return []string{
		"agentType", "authorId", "authorName", "builtIn", "organizationId",
		"createdAt", "updatedAt", "builtInAgentConfig", "suggestedPrompts",
		"systemPrompt", "llmModel", "llmApiKeyId", "isDefault", "scope",
		"teams", "considerContextUntrusted", "incomingEmailEnabled",
		"incomingEmailAllowedDomain", "incomingEmailSecurityMode", "icon",
		"slug", "tools",
	}
}

// mcpGatewayAttrSpec declares the wire shape for `archestra_mcp_gateway`. Same
// underlying agents table as archestra_agent / archestra_llm_proxy. No JSONB
// sub-objects — top-level columns only — but several Postgres `text[]`
// columns (knowledge_base_ids, connector_ids, passthrough_headers) which are
// atomic on the wire (Kind: List).
var mcpGatewayAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "knowledge_base_ids", JSONName: "knowledgeBaseIds", Kind: List},
	{TFName: "connector_ids", JSONName: "connectorIds", Kind: List},
	{TFName: "passthrough_headers", JSONName: "passthroughHeaders", Kind: List},
	{TFName: "identity_provider_id", JSONName: "identityProviderId", Kind: Scalar},
	{TFName: "consider_context_untrusted", JSONName: "considerContextUntrusted", Kind: Scalar},
	{TFName: "is_default", JSONName: "isDefault", Kind: Scalar},
	{TFName: "scope", JSONName: "scope", Kind: Scalar},
	{TFName: "teams", JSONName: "teams", Kind: List},
	{
		TFName: "labels", JSONName: "labels", Kind: Set,
		Children: []AttrSpec{
			{TFName: "key", JSONName: "key", Kind: Scalar},
			{TFName: "value", JSONName: "value", Kind: Scalar},
		},
	},
}
