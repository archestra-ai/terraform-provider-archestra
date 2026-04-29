package provider

import (
	"context"
	"encoding/json"
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
	// ToolIDByName is a name→UUID lookup table — same data as `tools`
	// but indexed for the common case of "I need this specific tool's
	// id." Lets users write
	// `archestra_mcp_server_installation.<n>.tool_id_by_name["<full-name>"]`
	// instead of either a `data "archestra_mcp_server_tool"` block or a
	// `for/if` HCL expression over the list.
	ToolIDByName types.Map `tfsdk:"tool_id_by_name"`
}

// mcpServerToolObjectType is the per-element shape of the `tools`
// Computed list. Surfaces every field the GetMcpServerTools wire returns
// that's useful in HCL: the {id, name, description} core for `for_each`,
// the JSON Schema `parameters` blob (as a string for dynamic-typed input
// validation in user code), the `assigned_agents` summary so users can
// see which agents already use the tool without a separate data source,
// and `created_at` for stable ordering.
var mcpServerToolAssignedAgentObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":   types.StringType,
	"name": types.StringType,
}}

var mcpServerToolObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                   types.StringType,
	"name":                 types.StringType,
	"description":          types.StringType,
	"parameters":           types.StringType,
	"assigned_agent_count": types.Int64Type,
	"assigned_agents":      types.ListType{ElemType: mcpServerToolAssignedAgentObjectType},
	"created_at":           types.StringType,
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
						"parameters": schema.StringAttribute{
							MarkdownDescription: "JSON Schema for the tool's input parameters, encoded as a JSON string. Use `jsondecode(t.parameters)` to introspect required fields, types, etc. for validation or downstream codegen.",
							Computed:            true,
						},
						"assigned_agent_count": schema.Int64Attribute{
							MarkdownDescription: "Number of agents this tool is currently assigned to. Quick visibility without fetching the full assignment list.",
							Computed:            true,
						},
						"assigned_agents": schema.ListNestedAttribute{
							MarkdownDescription: "Agents this tool is currently assigned to. Lets you see which agents already use a tool without a separate `data \"archestra_agent_tool\"` lookup per assignment.",
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										MarkdownDescription: "Agent UUID.",
										Computed:            true,
									},
									"name": schema.StringAttribute{
										MarkdownDescription: "Agent name (the agent's `name` field on `archestra_agent` / `archestra_llm_proxy` / `archestra_mcp_gateway`).",
										Computed:            true,
									},
								},
							},
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "RFC 3339 timestamp of when the tool was first registered with the backend. Useful as a stable sort key.",
							Computed:            true,
						},
					},
				},
			},
			"tool_id_by_name": schema.MapAttribute{
				MarkdownDescription: "Lookup table from each tool's wire name (`<server>__<short>`, e.g. `filesystem__read_text_file`) to its bare tool UUID. Same data as `tools[*].id`, indexed for the `tool_id = ...tool_id_by_name[\"<name>\"]` one-liner pattern (see the `archestra_agent_tool` example). " +
					"`null` while tools are still being discovered or the backend is unreachable; empty map `{}` when the install has booted but advertises no tools.",
				Computed:    true,
				ElementType: types.StringType,
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

	tools, toolIDsByName, toolsDiags := r.waitForServerTools(ctx, apiResp.JSON200.Id.String())
	resp.Diagnostics.Append(toolsDiags...)
	data.Tools = tools
	data.ToolIDByName = toolIDsByName

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
		flat, byName, projectDiags := projectMcpServerTools(*toolsResp.JSON200)
		resp.Diagnostics.Append(projectDiags...)
		if !projectDiags.HasError() {
			data.Tools = flat
			data.ToolIDByName = byName
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
// installed MCP server and its tool list is non-empty, then returns:
//
//   - `tools` — the flattened tool list (Computed `tools` attribute).
//   - `toolIDsByName` — name→uuid map (Computed `tool_id_by_name`),
//     same data, indexed for the common "look up this specific tool's
//     id" case. Both are populated from the same fetch so they can't
//     drift relative to each other.
//
// On timeout/error it returns properly-typed null values alongside a
// warning — callers downgrade so a slow scan doesn't fail the apply,
// the tools just appear on the next refresh.
func (r *MCPServerResource) waitForServerTools(ctx context.Context, serverID string) (types.List, types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	nullList := types.ListNull(mcpServerToolObjectType)
	nullMap := types.MapNull(types.StringType)

	serverUUID, err := uuid.Parse(serverID)
	if err != nil {
		diags.AddError("Invalid Server ID", fmt.Sprintf("failed to parse server ID: %s", err))
		return nullList, nullMap, diags
	}

	type toolsResult struct {
		list   types.List
		byName types.Map
	}
	res, found, err := RetryUntilFound(ctx, mcpServerRetryConfig, func() (toolsResult, bool, error) {
		toolsResp, err := r.client.GetMcpServerToolsWithResponse(ctx, serverUUID)
		if err != nil {
			return toolsResult{nullList, nullMap}, false, fmt.Errorf("failed to get server tools: %w", err)
		}
		if toolsResp.JSON200 == nil {
			return toolsResult{nullList, nullMap}, false, fmt.Errorf("unexpected response status: %d", toolsResp.StatusCode())
		}
		flat, byName, projectDiags := projectMcpServerTools(*toolsResp.JSON200)
		if projectDiags.HasError() {
			return toolsResult{nullList, nullMap}, false, fmt.Errorf("failed to project tools: %s", projectDiags)
		}
		// Treat empty list as "not yet ready" — the install just landed and
		// the MCP server hasn't responded to tools/list yet.
		ready := !flat.IsNull() && len(flat.Elements()) > 0
		return toolsResult{flat, byName}, ready, nil
	})
	// On every non-success path, substitute properly-typed null values:
	// `RetryUntilFound` returns the zero value of T on err / exhaustion,
	// and a zero-valued `types.List` / `types.Map` round-trips as
	// `tftypes.List[DynamicPseudoType]` etc. which the framework rejects
	// with a "MISSING TYPE" value-conversion error.
	if err != nil {
		diags.AddWarning("Tools fetch failed", err.Error())
		return nullList, nullMap, diags
	}
	if !found {
		diags.AddWarning("Tools not ready", "timeout waiting for MCP server tools to be ready")
		return nullList, nullMap, diags
	}
	return res.list, res.byName, diags
}

// projectMcpServerTools projects the `GetMcpServerTools` response onto
// both Computed attributes in one walk: the rich `tools` ListNested and
// the flat `tool_id_by_name` lookup map. Single function so the two
// can't drift relative to each other (same input, same iteration order).
//
// The list element exposes every wire field useful in HCL — id, name,
// description, JSON-encoded `parameters`, assigned-agent summary, and
// `created_at`. The map is just `name → id` for the common one-line
// lookup case.
//
// Duplicate wire-`name`s would silently collapse map entries; warn so
// users know data was lost. Backend convention is `<server>__<short>`
// which should be unique per install, but defending against future
// looseness is cheap.
func projectMcpServerTools(apiTools []struct {
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
}) (types.List, types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	listElements := make([]attr.Value, len(apiTools))
	mapEntries := make(map[string]attr.Value, len(apiTools))

	for i, t := range apiTools {
		desc := types.StringNull()
		if t.Description != nil {
			desc = types.StringValue(*t.Description)
		}

		// Parameters is JSON Schema; emit as a JSON string so HCL can
		// jsondecode() it. Empty / nil map → null, not "{}", to keep
		// state honest.
		params := types.StringNull()
		if len(t.Parameters) > 0 {
			if b, err := json.Marshal(t.Parameters); err == nil {
				params = types.StringValue(string(b))
			}
		}

		// Assigned agents — flat list of {id, name}.
		agentElems := make([]attr.Value, len(t.AssignedAgents))
		for j, a := range t.AssignedAgents {
			ao, d := types.ObjectValue(mcpServerToolAssignedAgentObjectType.AttrTypes, map[string]attr.Value{
				"id":   types.StringValue(a.Id),
				"name": types.StringValue(a.Name),
			})
			diags.Append(d...)
			if d.HasError() {
				return types.ListNull(mcpServerToolObjectType), types.MapNull(types.StringType), diags
			}
			agentElems[j] = ao
		}
		assignedAgents, d := types.ListValue(mcpServerToolAssignedAgentObjectType, agentElems)
		diags.Append(d...)
		if d.HasError() {
			return types.ListNull(mcpServerToolObjectType), types.MapNull(types.StringType), diags
		}

		obj, d := types.ObjectValue(mcpServerToolObjectType.AttrTypes, map[string]attr.Value{
			"id":                   types.StringValue(t.Id),
			"name":                 types.StringValue(t.Name),
			"description":          desc,
			"parameters":           params,
			"assigned_agent_count": types.Int64Value(int64(t.AssignedAgentCount)),
			"assigned_agents":      assignedAgents,
			"created_at":           types.StringValue(t.CreatedAt.Format(time.RFC3339)),
		})
		diags.Append(d...)
		if d.HasError() {
			return types.ListNull(mcpServerToolObjectType), types.MapNull(types.StringType), diags
		}
		listElements[i] = obj

		if _, exists := mapEntries[t.Name]; exists {
			diags.AddWarning(
				"Duplicate tool name",
				fmt.Sprintf("`tool_id_by_name` collapsed two tools with the same name %q; the second overwrote the first. The full list at `tools` still shows both.", t.Name),
			)
		}
		mapEntries[t.Name] = types.StringValue(t.Id)
	}

	listValue, d := types.ListValue(mcpServerToolObjectType, listElements)
	diags.Append(d...)
	if d.HasError() {
		return types.ListNull(mcpServerToolObjectType), types.MapNull(types.StringType), diags
	}
	mapValue, d := types.MapValue(types.StringType, mapEntries)
	diags.Append(d...)
	if d.HasError() {
		return types.ListNull(mcpServerToolObjectType), types.MapNull(types.StringType), diags
	}
	return listValue, mapValue, diags
}
