package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ resource.Resource = &AgentToolResource{}
var _ resource.ResourceWithImportState = &AgentToolResource{}

func NewAgentToolResource() resource.Resource {
	return &AgentToolResource{}
}

type AgentToolResource struct {
	client *client.ClientWithResponses
}

type AgentToolResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	AgentID                  types.String `tfsdk:"agent_id"`
	ToolID                   types.String `tfsdk:"tool_id"`
	McpServerID              types.String `tfsdk:"mcp_server_id"`
	CredentialResolutionMode types.String `tfsdk:"credential_resolution_mode"`
}

func (r *AgentToolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_tool"
}

func (r *AgentToolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns a tool to an Archestra agent (any agent type) and configures its execution settings.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite ID of the agent-tool assignment (`agent_id:tool_id`)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent to assign the tool to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tool_id": schema.StringAttribute{
				MarkdownDescription: "ID of the tool to assign",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "ID of the MCP Server instance associated with this tool",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credential_resolution_mode": schema.StringAttribute{
				MarkdownDescription: "How credentials are resolved for this tool. One of `static`, `dynamic`, `enterprise_managed`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("static", "dynamic", "enterprise_managed"),
				},
			},
		},
	}
}

func (r *AgentToolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AgentToolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentToolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", fmt.Sprintf("Unable to parse agent_id: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	prior := tftypes.NewValue(req.Plan.Raw.Type(), nil)
	patch := MergePatch(ctx, req.Plan.Raw, prior, agentToolAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_agent_tool Create", patch, agentToolAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}

	// `resolveAtCallTime` is intentionally not exposed: the backend collapses
	// it into `credential_resolution_mode` ("dynamic" when true, "static"
	// otherwise) and never stores or echoes it.

	assignResp, err := r.client.AssignToolToAgentWithBodyWithResponse(ctx, agentUUID, toolUUID, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to assign tool to agent, got error: %s", err))
		return
	}

	if assignResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("AssignToolToAgent: Expected 200 OK, got status %d: %s", assignResp.StatusCode(), string(assignResp.Body)),
		)
		return
	}

	if _, found := r.findAndReadState(ctx, agentUUID, toolUUID, &data, &resp.Diagnostics); !found {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Not Found", "Agent-tool assignment not found after creation")
		}
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%s:%s", data.AgentID.ValueString(), data.ToolID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
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

	toolUUID, err := uuid.Parse(parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	if _, found := r.findAndReadState(ctx, agentUUID, toolUUID, &data, &resp.Diagnostics); !found {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentToolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentToolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", fmt.Sprintf("Unable to parse agent_id: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, agentToolAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_agent_tool Update", patch, agentToolAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}

	assignmentID, found := r.findAndReadState(ctx, agentUUID, toolUUID, &data, &resp.Diagnostics)
	if !found {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Not Found", "Agent-tool assignment not found")
		}
		return
	}

	updateResp, err := r.client.UpdateAgentToolWithBodyWithResponse(ctx, assignmentID, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update agent tool, got error: %s", err))
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", updateResp.StatusCode()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentToolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", fmt.Sprintf("Unable to parse agent_id: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	delResp, err := r.client.UnassignToolFromAgentWithResponse(ctx, agentUUID, toolUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to unassign tool, got error: %s", err))
		return
	}

	if delResp.StatusCode() != 200 && delResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", delResp.StatusCode()),
		)
		return
	}
}

func (r *AgentToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// findAndReadState fetches agent tools in a single API call, finds the
// matching tool, populates the model, and returns the assignment UUID for use
// in Update calls.
func (r *AgentToolResource) findAndReadState(
	ctx context.Context,
	agentUUID, toolUUID uuid.UUID,
	data *AgentToolResourceModel,
	diags *diag.Diagnostics,
) (openapi_types.UUID, bool) {
	limit := 100
	offset := 0

	for {
		params := &client.GetAllAgentToolsParams{
			AgentId: &agentUUID,
			Limit:   &limit,
			Offset:  &offset,
		}

		resp, err := r.client.GetAllAgentToolsWithResponse(ctx, params)
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to read agent tool: %s", err))
			return uuid.Nil, false
		}

		if resp.JSON200 == nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to read agent tool: unexpected status code %d", resp.StatusCode()))
			return uuid.Nil, false
		}

		for i := range resp.JSON200.Data {
			at := &resp.JSON200.Data[i]
			if at.Tool.Id == toolUUID.String() {
				data.AgentID = types.StringValue(at.Agent.Id)
				data.ToolID = types.StringValue(at.Tool.Id)
				data.CredentialResolutionMode = types.StringValue(string(at.CredentialResolutionMode))

				if at.McpServerId != nil {
					data.McpServerID = types.StringValue(at.McpServerId.String())
				} else {
					data.McpServerID = types.StringNull()
				}
				return at.Id, true
			}
		}

		if !resp.JSON200.Pagination.HasNext {
			break
		}
		offset += limit
	}

	return uuid.Nil, false
}
