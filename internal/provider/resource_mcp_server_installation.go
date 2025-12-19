package provider

import (
	"context"
	"fmt"

	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	mcpServerPollTimeout  = 30 * time.Second
	mcpServerPollInterval = 1 * time.Second
)

var _ resource.Resource = &MCPServerResource{}
var _ resource.ResourceWithImportState = &MCPServerResource{}

func NewMCPServerResource() resource.Resource {
	return &MCPServerResource{}
}

type MCPServerResource struct {
	client *client.ClientWithResponses
}

type MCPServerResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	MCPServerID types.String `tfsdk:"mcp_server_id"`
}

func (r *MCPServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server_installation"
}

func (r *MCPServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra MCP server installation.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "MCP server identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the MCP server installation.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The actual name of the MCP server installation as returned by the API. The API may append a suffix to ensure uniqueness.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mcp_server_id": schema.StringAttribute{
				MarkdownDescription: "The MCP server ID from the private MCP registry (archestra_mcp_server resource)",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *MCPServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MCPServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MCPServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body using generated type
	requestBody := client.InstallMcpServerJSONRequestBody{
		Name: data.Name.ValueString(),
	}

	if !data.MCPServerID.IsNull() {
		mcpServerID, err := uuid.Parse(data.MCPServerID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid MCP Server ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
			return
		}
		requestBody.CatalogId = mcpServerID
	}

	// Call API
	apiResp, err := r.client.InstallMcpServerWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to install MCP server, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.DisplayName = types.StringValue(apiResp.JSON200.Name)
	data.MCPServerID = types.StringValue(apiResp.JSON200.CatalogId.String())

	if err := r.waitForServerTools(ctx, apiResp.JSON200.Id.String()); err != nil {
		resp.Diagnostics.AddWarning(
			"MCP Server Not Fully Ready",
			fmt.Sprintf("Server created successfully but tools are not yet available. They may appear shortly. Error: %s", err),
		)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MCPServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	serverID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.GetMcpServerWithResponse(ctx, serverID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read MCP server, got error: %s", err))
		return
	}

	// Handle not found
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	// Note: Keep user's configured name, set display_name to the API-returned name
	data.DisplayName = types.StringValue(apiResp.JSON200.Name)
	data.MCPServerID = types.StringValue(apiResp.JSON200.CatalogId.String())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// NOTE: The Archestra API does not support updating MCP servers.
	// Updates will trigger resource replacement (delete + create).
	// This function should never be called due to RequiresReplace plan modifiers on all attributes.
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"MCP server updates are not supported by the API. This should have triggered a replacement.",
	)
}

func (r *MCPServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MCPServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	serverID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.DeleteMcpServerWithResponse(ctx, serverID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete MCP server, got error: %s", err))
		return
	}

	// Check response (200 or 404 are both acceptable for delete)
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *MCPServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *MCPServerResource) waitForServerTools(ctx context.Context, serverID string) error {
	ctx, cancel := context.WithTimeout(ctx, mcpServerPollTimeout)
	defer cancel()

	ticker := time.NewTicker(mcpServerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for MCP server tools to be ready")

		case <-ticker.C:
			ready, err := r.checkServerToolsReady(ctx, serverID)
			if err != nil {
				tflog.Debug(ctx, "Error checking server tools", map[string]interface{}{
					"error": err.Error(),
				})
				continue
			}

			if ready {
				tflog.Info(ctx, "MCP server tools are ready", map[string]interface{}{
					"server_id": serverID,
				})
				return nil
			}

			tflog.Debug(ctx, "MCP server tools not yet ready, retrying...", map[string]interface{}{
				"server_id": serverID,
			})
		}
	}
}

func (r *MCPServerResource) checkServerToolsReady(ctx context.Context, serverID string) (bool, error) {
	toolsResp, err := r.client.GetToolsWithResponse(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get tools: %w", err)
	}

	if toolsResp.JSON200 == nil {
		return false, fmt.Errorf("unexpected response status: %d", toolsResp.StatusCode())
	}

	for _, tool := range *toolsResp.JSON200 {
		if tool.McpServer != nil && tool.McpServer.Id == serverID {
			return true, nil
		}
	}

	return false, nil
}
