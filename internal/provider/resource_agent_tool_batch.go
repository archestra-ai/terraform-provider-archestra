package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var (
	_ resource.Resource                = &AgentToolBatchResource{}
	_ resource.ResourceWithImportState = &AgentToolBatchResource{}
)

func NewAgentToolBatchResource() resource.Resource {
	return &AgentToolBatchResource{}
}

type AgentToolBatchResource struct {
	client *client.ClientWithResponses
}

// AgentToolBatchResourceModel models a one-shot bulk assignment of every
// tool from one MCP server installation onto one agent. The resource
// owns the (agent_id, mcp_server_id) tuple — it manages exactly the
// assignments that match both, and ignores any other assignments on the
// agent. Two batch resources targeting the same agent but different
// installs coexist without conflict.
type AgentToolBatchResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	AgentID                  types.String `tfsdk:"agent_id"`
	McpServerID              types.String `tfsdk:"mcp_server_id"`
	ToolIDs                  types.Set    `tfsdk:"tool_ids"`
	CredentialResolutionMode types.String `tfsdk:"credential_resolution_mode"`
}

func (r *AgentToolBatchResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_tool_batch"
}

func (r *AgentToolBatchResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Bulk-assigns a set of tools from one MCP server installation onto one agent in a single backend round-trip. Authoritative over `(agent_id, mcp_server_id)` — don't mix with `archestra_agent_tool` for the same pair.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite identifier `<agent_id>:<mcp_server_id>` — purely a Terraform-state token; not a backend resource ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "Agent UUID. Pass the `id` from `archestra_agent` / `archestra_llm_proxy` / `archestra_mcp_gateway`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "MCP server installation UUID. Pass `archestra_mcp_server_installation.<n>.id`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tool_ids": schema.SetAttribute{
				MarkdownDescription: "Set of bare tool UUIDs to assign. Typically `[for t in archestra_mcp_server_installation.<n>.tools : t.id]`. Adding members triggers a bulk-assign on the new ones only; removing members unassigns each individually. **Patched in-place — does not force replacement.**",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"credential_resolution_mode": schema.StringAttribute{
				MarkdownDescription: "How the agent resolves credentials when calling tools in this batch. Same semantics as `archestra_agent_tool.credential_resolution_mode`. Defaults to `static`.\n\n" +
					"~> **Asymmetric replacement.** Changing this value forces the resource to be replaced (drops every assignment and recreates), because the credential mode is per-assignment metadata that the bulk endpoint can't patch in place. Adding/removing entries in `tool_ids` is patched in-place — only this attribute, `agent_id`, and `mcp_server_id` trigger replacement.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("static"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("static", "dynamic", "enterprise_managed"),
				},
			},
		},
	}
}

func (r *AgentToolBatchResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AgentToolBatchResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AgentToolBatchResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, mcpUUID, toolIDs, ok := r.parsePlan(ctx, &plan, &resp.Diagnostics)
	if !ok {
		return
	}

	if err := r.bulkAssign(ctx, agentUUID, mcpUUID, toolIDs, plan.CredentialResolutionMode.ValueString()); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Bulk assign failed: %s", err))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", agentUUID, mcpUUID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentToolBatchResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentToolBatchResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", err.Error())
		return
	}
	mcpUUID, err := uuid.Parse(data.McpServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid mcp_server_id", err.Error())
		return
	}

	live, err := r.listAssignments(ctx, agentUUID, mcpUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent tools: %s", err))
		return
	}
	if len(live) == 0 {
		// All assignments removed externally — drop the resource.
		resp.State.RemoveResource(ctx)
		return
	}

	elems := make([]string, 0, len(live))
	for _, l := range live {
		elems = append(elems, l.toolID.String())
	}
	set, diags := types.SetValueFrom(ctx, types.StringType, elems)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	data.ToolIDs = set

	// Populate credential_resolution_mode from the live assignments —
	// per-assignment metadata that's the same across the batch by
	// construction. Without this, post-import state has the field null
	// while the schema's Default("static") plans "static", and the
	// resulting diff triggers RequiresReplace.
	if len(live) > 0 {
		data.CredentialResolutionMode = types.StringValue(live[0].credentialResolutionMode)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentToolBatchResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state AgentToolBatchResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, mcpUUID, planTools, ok := r.parsePlan(ctx, &plan, &resp.Diagnostics)
	if !ok {
		return
	}

	stateTools := parseUUIDSet(ctx, state.ToolIDs, &resp.Diagnostics, "state.tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}

	add, remove := diffUUIDSets(planTools, stateTools)

	if len(add) > 0 {
		if err := r.bulkAssign(ctx, agentUUID, mcpUUID, add, plan.CredentialResolutionMode.ValueString()); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Bulk assign (add) failed: %s", err))
			return
		}
	}
	for _, toolID := range remove {
		unassignResp, err := r.client.UnassignToolFromAgentWithResponse(ctx, agentUUID, toolID)
		if err != nil {
			resp.Diagnostics.AddError("API Error",
				fmt.Sprintf("Unassign tool %s failed: %s", toolID, err))
			return
		}
		if unassignResp.StatusCode() != 200 && unassignResp.StatusCode() != 204 && unassignResp.StatusCode() != 404 {
			resp.Diagnostics.AddError("API Error",
				fmt.Sprintf("Unassign tool %s returned status %d: %s",
					toolID, unassignResp.StatusCode(), string(unassignResp.Body)))
			return
		}
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentToolBatchResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentToolBatchResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", err.Error())
		return
	}

	tools := parseUUIDSet(ctx, data.ToolIDs, &resp.Diagnostics, "tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}
	for _, t := range tools {
		unassignResp, err := r.client.UnassignToolFromAgentWithResponse(ctx, agentUUID, t)
		if err != nil {
			resp.Diagnostics.AddError("API Error",
				fmt.Sprintf("Unassign tool %s failed: %s", t, err))
			return
		}
		if unassignResp.StatusCode() != 200 && unassignResp.StatusCode() != 204 && unassignResp.StatusCode() != 404 {
			resp.Diagnostics.AddError("API Error",
				fmt.Sprintf("Unassign tool %s returned status %d: %s",
					t, unassignResp.StatusCode(), string(unassignResp.Body)))
			return
		}
	}
}

func (r *AgentToolBatchResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: "<agent_id>:<mcp_server_id>". Tool IDs come from Read().
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID",
			"Expected `<agent_id>:<mcp_server_id>`")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mcp_server_id"), parts[1])...)
}

// --- helpers ---

func (r *AgentToolBatchResource) parsePlan(
	ctx context.Context,
	plan *AgentToolBatchResourceModel,
	diags *diag.Diagnostics,
) (agentUUID, mcpUUID openapi_types.UUID, toolIDs []openapi_types.UUID, ok bool) {
	a, err := uuid.Parse(plan.AgentID.ValueString())
	if err != nil {
		diags.AddError("Invalid agent_id", err.Error())
		return
	}
	m, err := uuid.Parse(plan.McpServerID.ValueString())
	if err != nil {
		diags.AddError("Invalid mcp_server_id", err.Error())
		return
	}
	tids := parseUUIDSet(ctx, plan.ToolIDs, diags, "tool_ids")
	if diags.HasError() {
		return
	}
	return a, m, tids, true
}

func (r *AgentToolBatchResource) bulkAssign(
	ctx context.Context,
	agentUUID openapi_types.UUID,
	mcpUUID openapi_types.UUID,
	toolIDs []openapi_types.UUID,
	credentialMode string,
) error {
	if len(toolIDs) == 0 {
		return nil
	}

	mcpStr := mcpUUID.String()
	var credPtr *client.BulkAssignToolsJSONBodyAssignmentsCredentialResolutionMode
	if credentialMode != "" {
		c := client.BulkAssignToolsJSONBodyAssignmentsCredentialResolutionMode(credentialMode)
		credPtr = &c
	}

	// Build the typed body. The element shape is the inline anonymous
	// struct on `BulkAssignToolsJSONBody.Assignments` — Go lets us
	// initialise it via a struct literal because the anonymous fields are
	// in the same package.
	body := client.BulkAssignToolsJSONRequestBody{
		Assignments: make([]struct {
			AgentId                  openapi_types.UUID                                                 `json:"agentId"`
			CredentialResolutionMode *client.BulkAssignToolsJSONBodyAssignmentsCredentialResolutionMode `json:"credentialResolutionMode,omitempty"`
			McpServerId              *openapi_types.UUID                                                `json:"mcpServerId"`
			ResolveAtCallTime        *bool                                                              `json:"resolveAtCallTime,omitempty"`
			ToolId                   openapi_types.UUID                                                 `json:"toolId"`
		}, 0, len(toolIDs)),
	}
	_ = mcpStr // kept for diagnostics readability; not used now
	for _, t := range toolIDs {
		body.Assignments = append(body.Assignments, struct {
			AgentId                  openapi_types.UUID                                                 `json:"agentId"`
			CredentialResolutionMode *client.BulkAssignToolsJSONBodyAssignmentsCredentialResolutionMode `json:"credentialResolutionMode,omitempty"`
			McpServerId              *openapi_types.UUID                                                `json:"mcpServerId"`
			ResolveAtCallTime        *bool                                                              `json:"resolveAtCallTime,omitempty"`
			ToolId                   openapi_types.UUID                                                 `json:"toolId"`
		}{
			AgentId:                  agentUUID,
			ToolId:                   t,
			McpServerId:              &mcpUUID,
			CredentialResolutionMode: credPtr,
		})
	}

	resp, err := r.client.BulkAssignToolsWithResponse(ctx, body)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("bulk-assign returned status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	if len(resp.JSON200.Failed) > 0 {
		failed := make([]string, 0, len(resp.JSON200.Failed))
		for _, f := range resp.JSON200.Failed {
			failed = append(failed, fmt.Sprintf("%s (%s)", f.ToolId, f.Error))
		}
		return fmt.Errorf("bulk-assign reported %d failure(s): %v", len(resp.JSON200.Failed), failed)
	}
	return nil
}

type liveAssignment struct {
	toolID                   openapi_types.UUID
	credentialResolutionMode string
}

func (r *AgentToolBatchResource) listAssignments(
	ctx context.Context,
	agentUUID, mcpUUID openapi_types.UUID,
) ([]liveAssignment, error) {
	limit := 100
	offset := 0
	var out []liveAssignment
	for {
		resp, err := r.client.GetAllAgentToolsWithResponse(ctx, &client.GetAllAgentToolsParams{
			AgentId: &agentUUID,
			Limit:   &limit,
			Offset:  &offset,
		})
		if err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("status %d: %s", resp.StatusCode(), string(resp.Body))
		}
		for i := range resp.JSON200.Data {
			at := &resp.JSON200.Data[i]
			if at.McpServerId == nil || *at.McpServerId != mcpUUID {
				continue
			}
			toolUUID, err := uuid.Parse(at.Tool.Id)
			if err != nil {
				continue
			}
			out = append(out, liveAssignment{
				toolID:                   toolUUID,
				credentialResolutionMode: string(at.CredentialResolutionMode),
			})
		}
		if !resp.JSON200.Pagination.HasNext {
			break
		}
		offset += limit
	}
	return out, nil
}
