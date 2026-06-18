package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var (
	_ resource.Resource                = &AgentDelegationResource{}
	_ resource.ResourceWithImportState = &AgentDelegationResource{}
)

func NewAgentDelegationResource() resource.Resource {
	return &AgentDelegationResource{}
}

type AgentDelegationResource struct {
	client *client.ClientWithResponses
}

// AgentDelegationResourceModel owns the per-agent delegation set: which
// other internal agents this agent may sub-delegate work to. Authoritative
// over the agent's full delegation list — the backend sync endpoint
// replaces all existing delegations with the new set.
type AgentDelegationResourceModel struct {
	ID             types.String `tfsdk:"id"`
	AgentID        types.String `tfsdk:"agent_id"`
	TargetAgentIDs types.Set    `tfsdk:"target_agent_ids"`
}

func (r *AgentDelegationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_delegation"
}

func (r *AgentDelegationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Delegation targets for a single agent — the set of other internal agents this agent may sub-delegate work to. Authoritative over the agent's full delegation list. Only `archestra_agent` and `archestra_mcp_gateway` accept delegations (LLM proxies reject with 400).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier — same as `agent_id`; surfaced as Computed so import-by-agent-id works cleanly.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "Source agent UUID — the agent that may delegate. Pass `archestra_agent.<n>.id` or `archestra_mcp_gateway.<n>.id`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"target_agent_ids": schema.SetAttribute{
				MarkdownDescription: "Bare UUIDs of internal agents this agent may delegate to. Self-delegation is rejected by the backend.",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
	}
}

func (r *AgentDelegationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *AgentDelegationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AgentDelegationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(plan.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", err.Error())
		return
	}

	targetUUIDs, ok := parseTargetAgentIDs(ctx, plan.TargetAgentIDs, &resp.Diagnostics)
	if !ok {
		return
	}

	if err := r.syncDelegations(ctx, agentUUID, targetUUIDs); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to sync delegations: %s", err))
		return
	}

	plan.ID = plan.AgentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentDelegationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentDelegationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", err.Error())
		return
	}

	apiResp, err := r.client.GetAgentDelegationsWithResponse(ctx, agentUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read delegations: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		// Source agent deleted out-of-band — drop the resource so the next
		// apply re-creates (or removes) the delegation set cleanly.
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	live := make([]attr.Value, 0, len(*apiResp.JSON200))
	for _, d := range *apiResp.JSON200 {
		live = append(live, types.StringValue(d.Id.String()))
	}
	setVal, diags := types.SetValue(types.StringType, live)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.TargetAgentIDs = setVal
	data.ID = data.AgentID

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentDelegationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AgentDelegationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(plan.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", err.Error())
		return
	}

	targetUUIDs, ok := parseTargetAgentIDs(ctx, plan.TargetAgentIDs, &resp.Diagnostics)
	if !ok {
		return
	}

	if err := r.syncDelegations(ctx, agentUUID, targetUUIDs); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to sync delegations: %s", err))
		return
	}

	plan.ID = plan.AgentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentDelegationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentDelegationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", err.Error())
		return
	}

	// Delete = sync to empty list. The backend's sync endpoint accepts
	// an empty array as "drop all delegations", which matches the
	// "delete the resource" semantic.
	if err := r.syncDelegations(ctx, agentUUID, nil); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to clear delegations: %s", err))
		return
	}
}

// ImportState accepts the bare `agent_id` UUID. The resource is uniquely
// identified by its source agent — there's at most one delegation set
// per agent, so a composite isn't needed.
func (r *AgentDelegationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := uuid.Parse(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid Import ID",
			fmt.Sprintf("Expected a UUID matching the source agent_id, got %q: %s", req.ID, err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), req.ID)...)
}

func (r *AgentDelegationResource) syncDelegations(ctx context.Context, agentUUID openapi_types.UUID, targets []openapi_types.UUID) error {
	// Backend's zod schema rejects null for `targetAgentIds` — pass an
	// explicit empty slice when there are no targets (the clear-all case
	// from Delete and the "drop the last delegation" Update path).
	if targets == nil {
		targets = []openapi_types.UUID{}
	}
	body := client.SyncAgentDelegationsJSONRequestBody{TargetAgentIds: targets}
	apiResp, err := r.client.SyncAgentDelegationsWithResponse(ctx, agentUUID, body)
	if err != nil {
		return err
	}
	if apiResp.JSON200 == nil {
		return fmt.Errorf("unexpected response status %d: %s", apiResp.StatusCode(), string(apiResp.Body))
	}
	return nil
}

// parseTargetAgentIDs turns a Set of String attributes into []UUID, sorted
// for stable wire ordering. Bad UUIDs raise a diagnostic and short-circuit.
func parseTargetAgentIDs(ctx context.Context, set types.Set, diags *diag.Diagnostics) ([]openapi_types.UUID, bool) {
	if set.IsNull() || set.IsUnknown() {
		return nil, true
	}
	var strs []string
	if d := set.ElementsAs(ctx, &strs, false); d.HasError() {
		diags.Append(d...)
		return nil, false
	}
	sort.Strings(strs)
	out := make([]openapi_types.UUID, len(strs))
	for i, s := range strs {
		u, err := uuid.Parse(s)
		if err != nil {
			diags.AddError("Invalid target_agent_ids entry", fmt.Sprintf("Could not parse %q as a UUID: %s", s, err))
			return nil, false
		}
		out[i] = u
	}
	return out, true
}
