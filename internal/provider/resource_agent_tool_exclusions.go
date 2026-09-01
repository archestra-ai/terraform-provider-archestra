package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var (
	_ resource.Resource                = &AgentToolExclusionsResource{}
	_ resource.ResourceWithImportState = &AgentToolExclusionsResource{}
)

func NewAgentToolExclusionsResource() resource.Resource { return &AgentToolExclusionsResource{} }

type AgentToolExclusionsResource struct {
	client *client.ClientWithResponses
}

// AgentToolExclusionsResourceModel models the FULL Auto-tool-mode exclusion
// list of one agent. The backend endpoint is a full replace, so the resource
// is authoritative: at most one per agent, and out-of-band additions are
// reverted on the next apply.
type AgentToolExclusionsResourceModel struct {
	ID              types.String `tfsdk:"id"`
	AgentID         types.String `tfsdk:"agent_id"`
	ExcludedToolIds types.Set    `tfsdk:"excluded_tool_ids"`
}

func (r *AgentToolExclusionsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_tool_exclusions"
}

func (r *AgentToolExclusionsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Auto-tool-mode exclusions for one agent: tools removed from its surface while `access_all_tools` is on (exclusions persist, inert, when it is off). **Authoritative over the agent's entire exclusion list** — every apply replaces the full set, and destroy clears it. Declare at most one per agent.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `agent_id` — the exclusion list has no id of its own",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"agent_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Agent UUID. Pass the `id` from `archestra_agent` / `archestra_mcp_gateway`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"excluded_tool_ids": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Bare tool UUIDs excluded from the agent's surface. An empty set is valid and clears all exclusions.",
			},
		},
	}
}

func (r *AgentToolExclusionsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AgentToolExclusionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AgentToolExclusionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.replaceExclusions(ctx, &plan, &resp.Diagnostics) {
		return
	}
	plan.ID = plan.AgentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentToolExclusionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentToolExclusionsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.GetAgentToolExclusionsWithResponse(ctx, agentUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent tool exclusions: %s", err))
		return
	}
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	ids := make([]string, len(apiResp.JSON200.ExcludedToolIds))
	for i, id := range apiResp.JSON200.ExcludedToolIds {
		ids[i] = id.String()
	}
	set, d := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(d...)
	data.ExcludedToolIds = set
	data.ID = data.AgentID

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentToolExclusionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AgentToolExclusionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.replaceExclusions(ctx, &plan, &resp.Diagnostics) {
		return
	}
	plan.ID = plan.AgentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete clears the agent's exclusion list — the resource owns the whole set,
// so destroy means "no exclusions", not "stop managing".
func (r *AgentToolExclusionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentToolExclusionsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.UpdateAgentToolExclusionsWithResponse(ctx, agentUUID,
		client.UpdateAgentToolExclusionsJSONRequestBody{ExcludedToolIds: []openapi_types.UUID{}})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to clear agent tool exclusions: %s", err))
		return
	}
	// Agent already gone → nothing to clear.
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

// ImportState imports by agent UUID (the list has no id of its own).
func (r *AgentToolExclusionsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), req.ID)...)
}

// AttrSpecs implements resourceWithAttrSpec (see specdrift_test.go). Both
// fields ride a typed body / the URL path rather than a merge patch, but the
// declaration keeps the schema↔spec drift lint active.
func (r *AgentToolExclusionsResource) AttrSpecs() []AttrSpec { return agentToolExclusionsAttrSpec }

func (r *AgentToolExclusionsResource) APIShape() any { return client.GetAgentToolExclusionsResponse{} }

func (r *AgentToolExclusionsResource) KnownIntentionallySkipped() []string { return nil }

var agentToolExclusionsAttrSpec = []AttrSpec{
	{TFName: "agent_id", Kind: Synthetic},
	{TFName: "excluded_tool_ids", JSONName: "excludedToolIds", Kind: Set},
}

// replaceExclusions PUTs the full plan-side exclusion set.
func (r *AgentToolExclusionsResource) replaceExclusions(ctx context.Context, plan *AgentToolExclusionsResourceModel, diags *diag.Diagnostics) bool {
	agentUUID, err := uuid.Parse(plan.AgentID.ValueString())
	if err != nil {
		diags.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return false
	}

	var rawIds []string
	diags.Append(plan.ExcludedToolIds.ElementsAs(ctx, &rawIds, false)...)
	if diags.HasError() {
		return false
	}
	toolIds := make([]openapi_types.UUID, len(rawIds))
	for i, raw := range rawIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			diags.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID %q: %s", raw, err))
			return false
		}
		toolIds[i] = id
	}

	apiResp, err := r.client.UpdateAgentToolExclusionsWithResponse(ctx, agentUUID,
		client.UpdateAgentToolExclusionsJSONRequestBody{ExcludedToolIds: toolIds})
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to replace agent tool exclusions: %s", err))
		return false
	}
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return false
	}
	return true
}
