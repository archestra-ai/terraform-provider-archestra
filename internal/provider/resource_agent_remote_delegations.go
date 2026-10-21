package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
	_ resource.Resource                = &AgentRemoteDelegationsResource{}
	_ resource.ResourceWithImportState = &AgentRemoteDelegationsResource{}
)

func NewAgentRemoteDelegationsResource() resource.Resource {
	return &AgentRemoteDelegationsResource{}
}

type AgentRemoteDelegationsResource struct {
	client *client.ClientWithResponses
}

// The endpoint replaces one agent's full external connection set.
type AgentRemoteDelegationsResourceModel struct {
	ID            types.String `tfsdk:"id"`
	AgentID       types.String `tfsdk:"agent_id"`
	ConnectionIds types.Set    `tfsdk:"connection_ids"`
}

func (r *AgentRemoteDelegationsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_remote_delegations"
}

func (r *AgentRemoteDelegationsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Authoritative external A2A subagent assignments for one agent. Declare one set per agent; apply replaces the full set and destroy clears assignments while preserving remote registrations.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `agent_id` — the assignment set has no id of its own",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"agent_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Agent UUID. Pass the `id` from `archestra_agent`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"connection_ids": schema.SetAttribute{
				Validators:          []validator.Set{setvalidator.SizeAtMost(500)},
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Remote connection UUIDs from archestra_remote_agent.connection_id; an empty set clears all external assignments.",
			},
		},
	}
}

func (r *AgentRemoteDelegationsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AgentRemoteDelegationsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AgentRemoteDelegationsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.replaceConnections(ctx, &plan, &resp.Diagnostics) {
		return
	}
	plan.ID = plan.AgentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentRemoteDelegationsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentRemoteDelegationsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.GetAgentA2aDelegationsWithResponse(ctx, agentUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent external A2A delegations: %s", err))
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

	ids := make([]string, len(*apiResp.JSON200))
	for i, id := range *apiResp.JSON200 {
		ids[i] = id.ConnectionId.String()
	}
	set, d := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(d...)
	data.ConnectionIds = set
	data.ID = data.AgentID

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentRemoteDelegationsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AgentRemoteDelegationsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.replaceConnections(ctx, &plan, &resp.Diagnostics) {
		return
	}
	plan.ID = plan.AgentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Clearing assignments preserves the separately managed remote registrations.
func (r *AgentRemoteDelegationsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentRemoteDelegationsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.SyncAgentA2aDelegationsWithResponse(ctx, agentUUID,
		client.SyncAgentA2aDelegationsJSONRequestBody{ConnectionIds: []openapi_types.UUID{}})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to clear agent external A2A delegations: %s", err))
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
func (r *AgentRemoteDelegationsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), req.ID)...)
}

// AttrSpecs implements resourceWithAttrSpec (see specdrift_test.go). Both
// fields ride a typed body / the URL path rather than a merge patch, but the
// declaration keeps the schema↔spec drift lint active.
func (r *AgentRemoteDelegationsResource) AttrSpecs() []AttrSpec {
	return agentRemoteDelegationsAttrSpec
}

func (r *AgentRemoteDelegationsResource) APIShape() any {
	return client.GetAgentA2aDelegationsResponse{}
}

// Connection IDs are aggregated in connection_ids; other target metadata belongs to the remote registration.
func (r *AgentRemoteDelegationsResource) KnownIntentionallySkipped() []string {
	return []string{"remoteAgentId", "connectionId", "toolId", "name", "description", "enabled"}
}

var agentRemoteDelegationsAttrSpec = []AttrSpec{
	{TFName: "agent_id", Kind: Synthetic},
	{TFName: "connection_ids", JSONName: "connectionIds", Kind: Set},
}

// The synchronization endpoint owns the complete connection set.
func (r *AgentRemoteDelegationsResource) replaceConnections(ctx context.Context, plan *AgentRemoteDelegationsResourceModel, diags *diag.Diagnostics) bool {
	agentUUID, err := uuid.Parse(plan.AgentID.ValueString())
	if err != nil {
		diags.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return false
	}

	var rawIds []string
	diags.Append(plan.ConnectionIds.ElementsAs(ctx, &rawIds, false)...)
	if diags.HasError() {
		return false
	}
	connectionIDs := make([]openapi_types.UUID, len(rawIds))
	for i, raw := range rawIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			diags.AddError("Invalid Connection ID", fmt.Sprintf("Unable to parse connection ID %q: %s", raw, err))
			return false
		}
		connectionIDs[i] = id
	}

	apiResp, err := r.client.SyncAgentA2aDelegationsWithResponse(ctx, agentUUID,
		client.SyncAgentA2aDelegationsJSONRequestBody{ConnectionIds: connectionIDs})
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to replace agent external A2A delegations: %s", err))
		return false
	}
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return false
	}

	observed, err := r.client.GetAgentA2aDelegationsWithResponse(ctx, agentUUID)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return false
	}
	if observed.JSON200 == nil {
		diags.AddError("Read Failed", fmt.Sprintf("API returned status %d", observed.StatusCode()))
		return false
	}
	persisted := make([]string, len(*observed.JSON200))
	for i, target := range *observed.JSON200 {
		persisted[i] = target.ConnectionId.String()
	}
	actual, d := types.SetValueFrom(ctx, types.StringType, persisted)
	diags.Append(d...)
	if !actual.Equal(plan.ConnectionIds) {
		diags.AddError("Invalid External Delegations", "The API did not persist the complete connection set.")
		return false
	}
	return !diags.HasError()
}
