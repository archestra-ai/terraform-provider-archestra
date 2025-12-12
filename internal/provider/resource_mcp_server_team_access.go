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

// Ensure interfaces are satisfied.
var _ resource.Resource = &McpServerTeamAccessResource{}
var _ resource.ResourceWithImportState = &McpServerTeamAccessResource{}

func NewMcpServerTeamAccessResource() resource.Resource {
	return &McpServerTeamAccessResource{}
}

type McpServerTeamAccessResource struct {
	client *client.ClientWithResponses
}

type McpServerTeamAccessResourceModel struct {
	ID          types.String `tfsdk:"id"`
	McpServerID types.String `tfsdk:"mcp_server_id"`
	TeamID      types.String `tfsdk:"team_id"`
}

func (r *McpServerTeamAccessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server_team_access"
}

func (r *McpServerTeamAccessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages team access to an MCP server.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier in format mcp_server_id/team_id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mcp_server_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the MCP server.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"team_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the team to grant access to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *McpServerTeamAccessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *McpServerTeamAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data McpServerTeamAccessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverUUID, err := uuid.Parse(data.McpServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Server ID", fmt.Sprintf("Could not parse server ID: %s", err))
		return
	}

	// FIXED: Using TeamIds (plural) and wrapping the single ID in a slice []string
	_, err = r.client.GrantTeamMcpServerAccessWithResponse(ctx, serverUUID, client.GrantTeamMcpServerAccessJSONRequestBody{
		TeamIds: []string{data.TeamID.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to grant team access: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%s/%s", data.McpServerID.ValueString(), data.TeamID.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpServerTeamAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpServerTeamAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverUUID, err := uuid.Parse(data.McpServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Server ID", fmt.Sprintf("Could not parse server ID: %s", err))
		return
	}

	apiResp, err := r.client.GetMcpServerWithResponse(ctx, serverUUID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read MCP server: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	teamHasAccess := false
	if apiResp.JSON200.Teams != nil {
		for _, teamId := range *apiResp.JSON200.Teams {
			if teamId == data.TeamID.ValueString() {
				teamHasAccess = true
				break
			}
		}
	}

	if !teamHasAccess {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpServerTeamAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpServerTeamAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverUUID, err := uuid.Parse(data.McpServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Server ID", fmt.Sprintf("Could not parse server ID: %s", err))
		return
	}

	// FIXED: Passing TeamID string directly.
	_, err = r.client.RevokeTeamMcpServerAccessWithResponse(ctx, serverUUID, data.TeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to revoke team access: %s", err))
		return
	}
}

func (r *McpServerTeamAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, "/")
	if len(idParts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected format: mcp_server_id/team_id, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mcp_server_id"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), idParts[1])...)
}

func (r *McpServerTeamAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No update logic needed
}
