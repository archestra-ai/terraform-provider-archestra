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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &LlmProxyResource{}
var _ resource.ResourceWithImportState = &LlmProxyResource{}

func NewLlmProxyResource() resource.Resource { return &LlmProxyResource{} }

type LlmProxyResource struct {
	client *client.ClientWithResponses
}

// LlmProxyResourceModel is the schema for an Archestra LLM proxy. The proxy
// fronts an upstream LLM provider and optionally enforces JWT auth via an
// identity provider.
type LlmProxyResourceModel struct {
	ID                       types.String      `tfsdk:"id"`
	Name                     types.String      `tfsdk:"name"`
	Description              types.String      `tfsdk:"description"`
	Icon                     types.String      `tfsdk:"icon"`
	LlmModel                 types.String      `tfsdk:"llm_model"`
	LlmApiKeyId              types.String      `tfsdk:"llm_api_key_id"`
	PassthroughHeaders       types.List        `tfsdk:"passthrough_headers"`
	IdentityProviderId       types.String      `tfsdk:"identity_provider_id"`
	ConsiderContextUntrusted types.Bool        `tfsdk:"consider_context_untrusted"`
	IsDefault                types.Bool        `tfsdk:"is_default"`
	Scope                    types.String      `tfsdk:"scope"`
	Teams                    types.List        `tfsdk:"teams"`
	Labels                   []AgentLabelModel `tfsdk:"labels"`
}

func (r *LlmProxyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_llm_proxy"
}

func (r *LlmProxyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra LLM proxy — a front-door to an upstream LLM provider, with optional inbound JWT auth via `identity_provider_id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "LLM proxy identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Proxy name"},
			"description": schema.StringAttribute{Optional: true, MarkdownDescription: "Human-readable description"},
			"icon":        schema.StringAttribute{Optional: true, MarkdownDescription: "Emoji or base64 image data URL"},
			"llm_model": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Upstream LLM model ID",
			},
			"llm_api_key_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ID of the upstream LLM provider API key",
			},
			"passthrough_headers": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Allowlist of HTTP header names to forward from proxy requests to the upstream LLM",
			},
			"identity_provider_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Identity provider used to validate inbound JWTs. Reference an `archestra_identity_provider`. Omit to disable JWT auth.",
			},
			"consider_context_untrusted": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the proxy context is treated as untrusted",
			},
			"is_default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this is the default LLM proxy",
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
				MarkdownDescription: "Team IDs this proxy is assigned to. Required when `scope = \"team\"`.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"labels": schema.SetNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Key/value labels for organizing proxies",
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

func (r *LlmProxyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LlmProxyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, llmProxyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	patch["agentType"] = "llm_proxy"
	LogPatch(ctx, "archestra_llm_proxy Create", patch, llmProxyAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal merge patch: %s", err))
		return
	}

	apiResp, err := r.client.CreateAgentWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create LLM proxy: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	var data LlmProxyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.flatten(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LlmProxyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LlmProxyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse LLM proxy ID: %s", err))
		return
	}

	apiResp, err := r.client.GetAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read LLM proxy, got error: %s", err))
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

func (r *LlmProxyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var stateData LlmProxyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(stateData.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse LLM proxy ID: %s", err))
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, llmProxyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_llm_proxy Update", patch, llmProxyAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal merge patch: %s", err))
		return
	}

	apiResp, err := r.client.UpdateAgentWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update LLM proxy: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	var data LlmProxyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.flatten(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LlmProxyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LlmProxyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse LLM proxy ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete LLM proxy, got error: %s", err))
		return
	}
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
	}
}

func (r *LlmProxyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *LlmProxyResource) flatten(ctx context.Context, data *LlmProxyResourceModel, body []byte, diags *diag.Diagnostics) {
	resp := parseAgentResponse(body, diags)
	if resp == nil {
		return
	}

	data.ID = types.StringValue(resp.Id.String())
	data.Name = types.StringValue(resp.Name)
	optionalStringFromAPI(&data.Description, resp.Description)
	optionalStringFromAPI(&data.Icon, resp.Icon)
	optionalStringFromAPI(&data.LlmModel, resp.LlmModel)
	optionalUUIDFromAPI(&data.LlmApiKeyId, resp.LlmApiKeyId)
	optionalStringFromAPI(&data.IdentityProviderId, resp.IdentityProviderId)

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
	data.Teams = teamsListFromAPI(ctx, resp.Teams, diags)

	data.Labels = flattenAgentLabels(data.Labels, resp.Labels)
}

// AttrSpecs implements resourceWithAttrSpec — activates the schema↔AttrSpec
// drift lint for this resource.
func (r *LlmProxyResource) AttrSpecs() []AttrSpec { return llmProxyAttrSpec }

// llmProxyAttrSpec declares the wire shape for `archestra_llm_proxy`. Same
// underlying agents table as archestra_agent (per
// platform/backend/src/database/schemas/agent.ts) so the column-storage
// rationale matches: no JSONB sub-objects, just top-level columns plus a
// Postgres `text[]` for passthrough_headers (atomic on the wire).
var llmProxyAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "llm_model", JSONName: "llmModel", Kind: Scalar},
	{TFName: "llm_api_key_id", JSONName: "llmApiKeyId", Kind: Scalar},
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
