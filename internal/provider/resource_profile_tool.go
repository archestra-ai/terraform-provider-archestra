package provider

import (
	"context"
	"fmt"
	"strings"

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

var _ resource.Resource = &ProfileToolResource{}
var _ resource.ResourceWithImportState = &ProfileToolResource{}

func NewProfileToolResource() resource.Resource {
	return &ProfileToolResource{}
}

type ProfileToolResource struct {
	client *client.ClientWithResponses
}

type ProfileToolResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	ProfileID                types.String `tfsdk:"profile_id"`
	ToolID                   types.String `tfsdk:"tool_id"`
	McpServerID              types.String `tfsdk:"mcp_server_id"`
	CredentialResolutionMode types.String `tfsdk:"credential_resolution_mode"`
}

func (r *ProfileToolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile_tool"
}

func (r *ProfileToolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns a tool to an Archestra Profile and configures its execution settings.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the profile-tool assignment (Composite ID: profile_id:tool_id)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the Profile to assign the tool to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tool_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the Tool to assign",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the MCP Server instance associated with this tool",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credential_resolution_mode": schema.StringAttribute{
				MarkdownDescription: "How credentials are resolved for this tool. Valid values: `static`, `dynamic`, `enterprise_managed`",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ProfileToolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ProfileToolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	profileUUID, err := uuid.Parse(data.ProfileID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	body := client.AssignToolToAgentJSONRequestBody{}

	if !data.McpServerID.IsNull() && !data.McpServerID.IsUnknown() {
		id, err := uuid.Parse(data.McpServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid MCP Server ID", fmt.Sprintf("Unable to parse ID: %s", err))
			return
		}
		body.McpServerId = &id
	}

	if !data.CredentialResolutionMode.IsNull() && !data.CredentialResolutionMode.IsUnknown() {
		mode := client.AssignToolToAgentJSONBodyCredentialResolutionMode(data.CredentialResolutionMode.ValueString())
		body.CredentialResolutionMode = &mode
	}

	assignResp, err := r.client.AssignToolToAgentWithResponse(ctx, profileUUID, toolUUID, body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to assign tool to profile, got error: %s", err))
		return
	}

	if assignResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("AssignToolToAgent: Expected 200 OK, got status %d: %s", assignResp.StatusCode(), string(assignResp.Body)),
		)
		return
	}

	if _, found := r.findAndReadState(ctx, profileUUID, toolUUID, &data, &resp.Diagnostics); !found {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Not Found", "Profile tool assignment not found after creation")
		}
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%s:%s", data.ProfileID.ValueString(), data.ToolID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parts := strings.Split(data.ID.ValueString(), ":")
	if len(parts) != 2 {
		resp.State.RemoveResource(ctx)
		return
	}

	profileUUID, err := uuid.Parse(parts[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	if _, found := r.findAndReadState(ctx, profileUUID, toolUUID, &data, &resp.Diagnostics); !found {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileToolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	profileUUID, err := uuid.Parse(data.ProfileID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	updateBody := client.UpdateAgentToolJSONRequestBody{}

	if !data.McpServerID.IsNull() {
		id, err := uuid.Parse(data.McpServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid MCP Server ID", fmt.Sprintf("Unable to parse ID: %s", err))
			return
		}
		updateBody.McpServerId = &id
	}

	if !data.CredentialResolutionMode.IsNull() {
		mode := client.UpdateAgentToolJSONBodyCredentialResolutionMode(data.CredentialResolutionMode.ValueString())
		updateBody.CredentialResolutionMode = &mode
	}

	assignmentID, found := r.findAndReadState(ctx, profileUUID, toolUUID, &data, &resp.Diagnostics)
	if !found {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Not Found", "Profile tool assignment not found")
		}
		return
	}

	updateResp, err := r.client.UpdateAgentToolWithResponse(ctx, assignmentID, updateBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update profile tool, got error: %s", err))
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

func (r *ProfileToolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProfileToolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	profileUUID, err := uuid.Parse(data.ProfileID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	toolUUID, err := uuid.Parse(data.ToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}

	delResp, err := r.client.UnassignToolFromAgentWithResponse(ctx, profileUUID, toolUUID)
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

func (r *ProfileToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// findAndReadState fetches profile tools in a single API call, finds the matching tool,
// populates the model, and returns the assignment UUID for use in Update calls.
func (r *ProfileToolResource) findAndReadState(
	ctx context.Context,
	profileUUID, toolUUID uuid.UUID,
	data *ProfileToolResourceModel,
	diags *diag.Diagnostics,
) (openapi_types.UUID, bool) {
	limit := 100
	offset := 0

	for {
		params := &client.GetAllAgentToolsParams{
			AgentId: &profileUUID,
			Limit:   &limit,
			Offset:  &offset,
		}

		resp, err := r.client.GetAllAgentToolsWithResponse(ctx, params)
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to read profile tool: %s", err))
			return uuid.Nil, false
		}

		if resp.JSON200 == nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to read profile tool: unexpected status code %d", resp.StatusCode()))
			return uuid.Nil, false
		}

		for i := range resp.JSON200.Data {
			at := &resp.JSON200.Data[i]
			if at.Tool.Id == toolUUID.String() {
				data.ProfileID = types.StringValue(at.Agent.Id)
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
