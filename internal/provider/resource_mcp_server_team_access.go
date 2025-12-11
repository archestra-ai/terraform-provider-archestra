package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	
	// We use this specific package for UUIDs to match the generated client
	openapi_types "github.com/oapi-codegen/runtime/types"

	// Using the correct long path
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// Ensure implementation of the interface
var (
	_ resource.Resource              = &McpServerTeamAccessResource{}
	_ resource.ResourceWithConfigure = &McpServerTeamAccessResource{}
)

func NewMcpServerTeamAccessResource() resource.Resource {
	return &McpServerTeamAccessResource{}
}

type McpServerTeamAccessResource struct {
	client *client.Client
}

type McpServerTeamAccessResourceModel struct {
	McpServerID types.String `tfsdk:"mcp_server_id"`
	TeamID      types.String `tfsdk:"team_id"`
}

func (r *McpServerTeamAccessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server_team_access"
}

func (r *McpServerTeamAccessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages team access to an MCP server.",
		Attributes: map[string]schema.Attribute{
			"mcp_server_id": schema.StringAttribute{
				Description:   "The ID of the MCP server.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"team_id": schema.StringAttribute{
				Description:   "The ID of the team.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *McpServerTeamAccessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
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

	// 1. Convert Server ID String -> UUID
	serverUUID, err := openapi_types.ParseUUID(data.McpServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Server ID", "mcp_server_id must be a valid UUID")
		return
	}

	// 2. Prepare the Body
	bodyStr := fmt.Sprintf(`{"team_id": "%s"}`, data.TeamID.ValueString())
	bodyReader := strings.NewReader(bodyStr)

	// 3. Call the Official Client
	res, err := r.client.GrantTeamMcpServerAccessWithBody(ctx, serverUUID, "application/json", bodyReader)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to grant access: %s", err))
		return
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("API returned status: %d", res.StatusCode))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpServerTeamAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpServerTeamAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpServerTeamAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Not needed due to RequiresReplace
}

func (r *McpServerTeamAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpServerTeamAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 1. Convert Server ID String -> UUID
	serverUUID, err := openapi_types.ParseUUID(data.McpServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Server ID", "mcp_server_id must be a valid UUID")
		return
	}

	// 2. Call Revoke
	res, err := r.client.RevokeTeamMcpServerAccess(ctx, serverUUID, data.TeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to revoke access: %s", err))
		return
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("API returned status: %d", res.StatusCode))
		return
	}
}