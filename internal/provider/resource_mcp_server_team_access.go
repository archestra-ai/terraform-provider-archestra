package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

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
				Description: "The ID of the MCP server.",
				Required:    true,
				// Changing the Server ID forces a new resource to be created
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"team_id": schema.StringAttribute{
				Description: "The ID of the team.",
				Required:    true,
				// Changing the Team ID forces a new resource to be created
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *McpServerTeamAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data McpServerTeamAccessResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call the API to Grant Access
	err := r.client.GrantTeamMcpServerAccess(ctx, data.McpServerID.ValueString(), data.TeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to grant team access to MCP server: %s", err))
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpServerTeamAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpServerTeamAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Since this is a simple link, we assume it exists if it is in state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpServerTeamAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This resource uses RequiresReplace, so Update is never called.
}

func (r *McpServerTeamAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpServerTeamAccessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call the API to Revoke Access
	err := r.client.RevokeTeamMcpServerAccess(ctx, data.McpServerID.ValueString(), data.TeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to revoke team access from MCP server: %s", err))
		return
	}
}