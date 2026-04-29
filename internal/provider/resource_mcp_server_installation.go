package provider

import (
	"context"
	"fmt"

	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var mcpServerRetryConfig = RetryConfig{
	MaxRetries:     30,
	InitialBackoff: 1 * time.Second,
	MaxBackoff:     2 * time.Second,
	Description:    "MCP server tools",
}

var _ resource.Resource = &MCPServerResource{}
var _ resource.ResourceWithImportState = &MCPServerResource{}

func NewMCPServerResource() resource.Resource {
	return &MCPServerResource{}
}

type MCPServerResource struct {
	client *client.ClientWithResponses
}

type MCPServerResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	DisplayName       types.String `tfsdk:"display_name"`
	CatalogID         types.String `tfsdk:"catalog_id"`
	TeamID            types.String `tfsdk:"team_id"`
	EnvironmentValues types.Map    `tfsdk:"environment_values"`
	UserConfigValues  types.Map    `tfsdk:"user_config_values"`
	SecretID          types.String `tfsdk:"secret_id"`
	AccessToken       types.String `tfsdk:"access_token"`
	ServiceAccount    types.String `tfsdk:"service_account"`
	IsByosVault       types.Bool   `tfsdk:"is_byos_vault"`
	AgentIDs          types.List   `tfsdk:"agent_ids"`
	// Tools is a Computed list — the slice form ([]struct) can't represent
	// the plan-time "unknown" marker the framework needs before Create
	// runs, so this stays a types.List wrapping mcpServerToolObjectType.
	Tools types.List `tfsdk:"tools"`
}

// mcpServerToolObjectType is the per-element shape of the `tools`
// Computed list. Kept narrow on purpose ({id, name, description}); the
// backend returns more (parameters, assignedAgents, …) but those drive
// separate resources or are debug-only.
var mcpServerToolObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":          types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
}}

func (r *MCPServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server_installation"
}

func (r *MCPServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra MCP server installation.\n\n" +
			"~> **Note:** The `ownerId` and `userId` fields on the underlying API are derived " +
			"from the authenticated caller and cannot be set declaratively. Any value sent in " +
			"the request body is overwritten by the backend with the API key's user ID, so " +
			"these fields are intentionally not exposed on this resource.",

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
			"catalog_id": schema.StringAttribute{
				MarkdownDescription: "Catalog item ID (UUID of the `archestra_mcp_registry_catalog_item` resource) this installation is based on.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Team ID for team-scoped installations",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_values": schema.MapAttribute{
				MarkdownDescription: "Environment variable values for the MCP server installation",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"user_config_values": schema.MapAttribute{
				MarkdownDescription: "User configuration field values for the MCP server installation",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"secret_id": schema.StringAttribute{
				MarkdownDescription: "Pre-created secret UUID for the MCP server installation",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_token": schema.StringAttribute{
				MarkdownDescription: "Personal access token for the MCP server",
				Optional:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account": schema.StringAttribute{
				MarkdownDescription: "Kubernetes service account override for the MCP server pod",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_byos_vault": schema.BoolAttribute{
				MarkdownDescription: "When true, environment_values and user_config_values are treated as vault references",
				Optional:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"agent_ids": schema.ListAttribute{
				MarkdownDescription: "Agent IDs to auto-assign tools to on install",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"tools": schema.ListNestedAttribute{
				MarkdownDescription: "Tools exposed by the installed MCP server. Populated after install (and refreshed on read) so you can fan out per-tool resources without separate `data \"archestra_mcp_server_tool\"` lookups:\n\n" +
					"```hcl\n" +
					"for_each = { for t in archestra_mcp_server_installation.<name>.tools : t.name => t }\n" +
					"```",
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Tool UUID. Use as `tool_id` on `archestra_tool_invocation_policy` / `archestra_trusted_data_policy`.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Tool name (the MCP server's own identifier — stable across installs of the same catalog item).",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "Human-readable description as advertised by the MCP server. May be null.",
							Computed:            true,
						},
					},
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

	catalogUUID, catalogErr := uuid.Parse(data.CatalogID.ValueString())
	if catalogErr != nil {
		resp.Diagnostics.AddError("Invalid catalog_id", fmt.Sprintf("Unable to parse catalog_id as a UUID: %s", catalogErr))
		return
	}
	requestBody.CatalogId = catalogUUID

	if !data.TeamID.IsNull() && !data.TeamID.IsUnknown() {
		teamId := data.TeamID.ValueString()
		requestBody.TeamId = &teamId
	}

	if !data.EnvironmentValues.IsNull() && !data.EnvironmentValues.IsUnknown() {
		var envVals map[string]string
		resp.Diagnostics.Append(data.EnvironmentValues.ElementsAs(ctx, &envVals, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.EnvironmentValues = &envVals
	}

	if !data.UserConfigValues.IsNull() && !data.UserConfigValues.IsUnknown() {
		var ucVals map[string]string
		resp.Diagnostics.Append(data.UserConfigValues.ElementsAs(ctx, &ucVals, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.UserConfigValues = &ucVals
	}

	if !data.SecretID.IsNull() && !data.SecretID.IsUnknown() {
		secretUUID, err := uuid.Parse(data.SecretID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Secret ID", fmt.Sprintf("Unable to parse secret ID: %s", err))
			return
		}
		requestBody.SecretId = &secretUUID
	}

	if !data.AccessToken.IsNull() && !data.AccessToken.IsUnknown() {
		token := data.AccessToken.ValueString()
		requestBody.AccessToken = &token
	}

	if !data.ServiceAccount.IsNull() && !data.ServiceAccount.IsUnknown() {
		sa := data.ServiceAccount.ValueString()
		requestBody.ServiceAccount = &sa
	}

	if !data.IsByosVault.IsNull() && !data.IsByosVault.IsUnknown() {
		isByosVault := data.IsByosVault.ValueBool()
		requestBody.IsByosVault = &isByosVault
	}

	if !data.AgentIDs.IsNull() && !data.AgentIDs.IsUnknown() {
		var agentIDStrs []string
		resp.Diagnostics.Append(data.AgentIDs.ElementsAs(ctx, &agentIDStrs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		agentUUIDs := make([]openapi_types.UUID, len(agentIDStrs))
		for i, idStr := range agentIDStrs {
			parsed, err := uuid.Parse(idStr)
			if err != nil {
				resp.Diagnostics.AddError("Invalid Agent ID", fmt.Sprintf("Unable to parse agent ID %q: %s", idStr, err))
				return
			}
			agentUUIDs[i] = parsed
		}
		requestBody.AgentIds = &agentUUIDs
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
	data.CatalogID = types.StringValue(apiResp.JSON200.CatalogId.String())

	if apiResp.JSON200.TeamId != nil {
		data.TeamID = types.StringValue(*apiResp.JSON200.TeamId)
	}

	tools, toolsDiags := r.waitForServerTools(ctx, apiResp.JSON200.Id.String())
	resp.Diagnostics.Append(toolsDiags...)
	data.Tools = tools

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
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server installation ID: %s", err))
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
	data.CatalogID = types.StringValue(apiResp.JSON200.CatalogId.String())

	if apiResp.JSON200.TeamId != nil {
		data.TeamID = types.StringValue(*apiResp.JSON200.TeamId)
	} else {
		data.TeamID = types.StringNull()
	}

	if apiResp.JSON200.SecretId != nil {
		data.SecretID = types.StringValue(apiResp.JSON200.SecretId.String())
	} else {
		data.SecretID = types.StringNull()
	}

	// EnvironmentValues, UserConfigValues, and AccessToken are write-only;
	// preserve from prior state to avoid spurious diffs.

	// Refresh tools — drift-honest per A7. The MCP server can advertise
	// new/removed tools at runtime; surfacing the change in plan is the
	// point. On fetch failure, fall back to prior state so a transient
	// backend hiccup doesn't blank the list.
	toolsResp, toolsErr := r.client.GetMcpServerToolsWithResponse(ctx, serverID)
	switch {
	case toolsErr != nil:
		resp.Diagnostics.AddWarning(
			"Tools Refresh Failed",
			fmt.Sprintf("Could not refresh tools list for MCP server %s: %s. Using last-known state.", serverID, toolsErr),
		)
	case toolsResp.JSON200 == nil:
		resp.Diagnostics.AddWarning(
			"Tools Refresh Returned Non-200",
			fmt.Sprintf("GetMcpServerTools returned status %d for server %s. Using last-known state.", toolsResp.StatusCode(), serverID),
		)
	default:
		flat, flatDiags := flattenMcpServerTools(*toolsResp.JSON200)
		resp.Diagnostics.Append(flatDiags...)
		if !flatDiags.HasError() {
			data.Tools = flat
		}
	}

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
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server installation ID: %s", err))
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

// waitForServerTools polls until the backend has finished scanning the
// installed MCP server and its tool list is non-empty, then returns the
// flattened tool list for the resource's `tools` Computed attribute.
// On timeout/error it returns whatever's been seen so far (often a null
// list) alongside the error — callers downgrade to a warning so a slow
// scan doesn't fail the apply, the tools just appear on the next refresh.
func (r *MCPServerResource) waitForServerTools(ctx context.Context, serverID string) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	serverUUID, err := uuid.Parse(serverID)
	if err != nil {
		diags.AddError("Invalid Server ID", fmt.Sprintf("failed to parse server ID: %s", err))
		return types.ListNull(mcpServerToolObjectType), diags
	}

	tools, found, err := RetryUntilFound(ctx, mcpServerRetryConfig, func() (types.List, bool, error) {
		toolsResp, err := r.client.GetMcpServerToolsWithResponse(ctx, serverUUID)
		if err != nil {
			return types.ListNull(mcpServerToolObjectType), false, fmt.Errorf("failed to get server tools: %w", err)
		}
		if toolsResp.JSON200 == nil {
			return types.ListNull(mcpServerToolObjectType), false, fmt.Errorf("unexpected response status: %d", toolsResp.StatusCode())
		}
		flat, flatDiags := flattenMcpServerTools(*toolsResp.JSON200)
		if flatDiags.HasError() {
			return types.ListNull(mcpServerToolObjectType), false, fmt.Errorf("failed to flatten tools: %s", flatDiags)
		}
		// Treat empty list as "not yet ready" — the install just landed and
		// the MCP server hasn't responded to tools/list yet.
		ready := !flat.IsNull() && len(flat.Elements()) > 0
		return flat, ready, nil
	})
	if err != nil {
		diags.AddWarning("Tools fetch failed", err.Error())
		return tools, diags
	}
	if !found {
		diags.AddWarning("Tools not ready", "timeout waiting for MCP server tools to be ready")
		return tools, diags
	}
	return tools, diags
}

// flattenMcpServerTools projects the GetMcpServerTools response items
// onto the resource's `tools` schema. Kept narrow to {id, name,
// description}: that's enough to drive the documented `for_each` pattern.
// Adding fields later is non-breaking; removing them is.
func flattenMcpServerTools(apiTools []struct {
	AssignedAgentCount float32 `json:"assignedAgentCount"`
	AssignedAgents     []struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"assignedAgents"`
	CreatedAt   time.Time              `json:"createdAt"`
	Description *string                `json:"description"`
	Id          string                 `json:"id"`
	Name        string                 `json:"name"`
	Parameters  map[string]interface{} `json:"parameters"`
}) (types.List, diag.Diagnostics) {
	elements := make([]attr.Value, len(apiTools))
	for i, t := range apiTools {
		desc := types.StringNull()
		if t.Description != nil {
			desc = types.StringValue(*t.Description)
		}
		obj, d := types.ObjectValue(mcpServerToolObjectType.AttrTypes, map[string]attr.Value{
			"id":          types.StringValue(t.Id),
			"name":        types.StringValue(t.Name),
			"description": desc,
		})
		if d.HasError() {
			return types.ListNull(mcpServerToolObjectType), d
		}
		elements[i] = obj
	}
	return types.ListValue(mcpServerToolObjectType, elements)
}
