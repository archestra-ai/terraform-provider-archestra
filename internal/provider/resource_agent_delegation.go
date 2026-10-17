package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &AgentDelegationResource{}
var _ resource.ResourceWithImportState = &AgentDelegationResource{}

func NewAgentDelegationResource() resource.Resource {
	return &AgentDelegationResource{}
}

type AgentDelegationResource struct {
	client *client.ClientWithResponses
}

type AgentDelegationResourceModel struct {
	ID            types.String `tfsdk:"id"`
	AgentID       types.String `tfsdk:"agent_id"`
	TargetAgentID types.String `tfsdk:"target_agent_id"`
}

func (r *AgentDelegationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_delegation"
}

func (r *AgentDelegationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Delegates one agent to another: the delegating agent surfaces the target as a subagent " +
			"delegation tool it can hand tasks to mid-conversation. One resource per edge; manage an agent's whole " +
			"delegation surface as several of these.\n\n" +
			"Agents, MCP gateways, and profiles may delegate (the `agent_id` side); the delegation **target** must be " +
			"an internal agent. LLM proxies cannot delegate at all.\n\n" +
			"Each resource adds or removes only its own edge, preserving other targets even when independent Terraform processes reconcile concurrently.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite ID of the delegation edge (`agent_id:target_agent_id`) — purely a Terraform-state token; not a backend resource ID",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the delegating agent (the one that gains the delegation tool)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"target_agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent tasks are delegated to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *AgentDelegationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentDelegationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", fmt.Sprintf("Unable to parse agent_id: %s", err))
		return
	}

	targetUUID, err := uuid.Parse(data.TargetAgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid target_agent_id", fmt.Sprintf("Unable to parse target_agent_id: %s", err))
		return
	}

	created, err := r.client.CreateAgentDelegationWithResponse(ctx, agentUUID, targetUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create agent delegation: %s", err))
		return
	}
	if created.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("CreateAgentDelegation: Expected 200 OK, got status %d: %s", created.StatusCode(), string(created.Body)))
		return
	}
	if !created.JSON200.Created {
		resp.Diagnostics.AddError("Delegation Already Exists", fmt.Sprintf("Agent %s already delegates to %s. Import it instead:\n\n  terraform import <address> %s:%s", agentUUID, targetUUID, agentUUID, targetUUID))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%s:%s", agentUUID, targetUUID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentDelegationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentDelegationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	parts := strings.Split(data.ID.ValueString(), ":")
	if len(parts) != 2 {
		resp.State.RemoveResource(ctx)
		return
	}

	agentUUID, err := uuid.Parse(parts[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", fmt.Sprintf("Unable to parse agent_id: %s", err))
		return
	}

	targetUUID, err := uuid.Parse(parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Invalid target_agent_id", fmt.Sprintf("Unable to parse target_agent_id: %s", err))
		return
	}

	// Write both halves back so post-import state matches the user's HCL —
	// they are Required+RequiresReplace; left null after Read, the next plan
	// diffs them and triggers destroy+recreate.
	data.AgentID = types.StringValue(agentUUID.String())
	data.TargetAgentID = types.StringValue(targetUUID.String())

	delResp, err := r.client.GetAgentDelegationsWithResponse(ctx, agentUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent delegations: %s", err))
		return
	}
	// The delegating agent itself is gone (or was never visible) — so is the edge.
	if IsNotFound(delResp) {
		resp.State.RemoveResource(ctx)
		return
	}
	if delResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("GetAgentDelegations: Expected 200 OK, got status %d: %s", delResp.StatusCode(), string(delResp.Body)),
		)
		return
	}

	for _, target := range *delResp.JSON200 {
		if target.Id == targetUUID {
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

// Update is unreachable: both attributes force replacement, so every change
// plans as destroy+create. The framework still requires the method.
func (r *AgentDelegationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentDelegationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentDelegationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentDelegationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", fmt.Sprintf("Unable to parse agent_id: %s", err))
		return
	}

	targetUUID, err := uuid.Parse(data.TargetAgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid target_agent_id", fmt.Sprintf("Unable to parse target_agent_id: %s", err))
		return
	}

	delResp, err := r.client.DeleteAgentDelegationWithResponse(ctx, agentUUID, targetUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to remove delegation, got error: %s", err))
		return
	}

	if delResp.StatusCode() != 200 && delResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", delResp.StatusCode(), string(delResp.Body)),
		)
		return
	}
}

func (r *AgentDelegationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected \"<agent_id>:<target_agent_id>\", got %q", req.ID),
		)
		return
	}
	for i, name := range []string{"agent_id", "target_agent_id"} {
		if _, err := uuid.Parse(parts[i]); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Import ID",
				fmt.Sprintf("The %s half of %q is not a UUID: %s", name, req.ID, err),
			)
			return
		}
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
