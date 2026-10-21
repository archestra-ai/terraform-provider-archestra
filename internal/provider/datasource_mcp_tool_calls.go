package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ datasource.DataSource = &McpToolCallsDataSource{}

func NewMcpToolCallsDataSource() datasource.DataSource {
	return &McpToolCallsDataSource{}
}

type McpToolCallsDataSource struct {
	client *client.ClientWithResponses
}

type McpToolCallsDataSourceModel struct {
	AgentID       types.String `tfsdk:"agent_id"`
	StartDate     types.String `tfsdk:"start_date"`
	EndDate       types.String `tfsdk:"end_date"`
	ServerName    types.String `tfsdk:"mcp_server_name"`
	MaxRecords    types.Int64  `tfsdk:"max_records"`
	Calls         types.List   `tfsdk:"calls"`
	ReturnedCount types.Int64  `tfsdk:"returned_count"`
	Truncated     types.Bool   `tfsdk:"truncated"`
}

// defaultMcpToolCallsMaxRecords caps how many records the data source
// will pull into Terraform state when the caller doesn't set
// `max_records` explicitly. Backend audit logs grow unboundedly; pulling
// every match into state on a misconfigured filter (e.g. `start_date`
// one year ago, no agent filter) can blow up state size and apply time.
// Callers who genuinely need everything can set `max_records` to a
// large explicit value.
const defaultMcpToolCallsMaxRecords int64 = 1000

// mcpToolCallObjectType is the per-element shape of `calls`. Mirrors the
// wire as closely as is HCL-friendly: arguments and result come back as
// JSON strings (use `jsondecode` to introspect), the rest are flat.
var mcpToolCallObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"app_id":          types.StringType,
	"app_name":        types.StringType,
	"oauth_client_id": types.StringType,
	"owner_type":      types.StringType,
	"run_id":          types.StringType,
	"source":          types.StringType,
	"tool_call_json":  types.StringType,
	"id":              types.StringType,
	"agent_id":        types.StringType,
	"mcp_server_name": types.StringType,
	"method":          types.StringType,
	"tool_name":       types.StringType,
	"tool_call_id":    types.StringType,
	"arguments":       types.StringType,
	"result":          types.StringType,
	"auth_method":     types.StringType,
	"user_id":         types.StringType,
	"user_name":       types.StringType,
	"created_at":      types.StringType,
}}

func (d *McpToolCallsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_tool_calls"
}

func (d *McpToolCallsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Queries the MCP tool-call audit log using cursor pagination, bounded by `max_records`.",

		Attributes: map[string]schema.Attribute{
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "Optional. Filter to calls made by this agent (UUID).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(uuidRegexp, "agent_id must be a UUID"),
				},
			},
			"start_date": schema.StringAttribute{
				MarkdownDescription: "Optional. RFC 3339 / ISO 8601 timestamp; only return calls created at or after this instant.",
				Optional:            true,
			},
			"end_date": schema.StringAttribute{
				MarkdownDescription: "Optional. RFC 3339 / ISO 8601 timestamp; only return calls created at or before this instant.",
				Optional:            true,
			},
			"mcp_server_name": schema.StringAttribute{
				MarkdownDescription: "Filter by exact MCP server name.",
				Optional:            true,
			},
			"max_records": schema.Int64Attribute{
				MarkdownDescription: fmt.Sprintf("Optional. Hard cap on records pulled into Terraform state, regardless of how many the backend's filter matches. Defaults to %d. `truncated` reports whether more matching records remain.", defaultMcpToolCallsMaxRecords),
				Optional:            true,
			},
			"returned_count": schema.Int64Attribute{
				MarkdownDescription: "Number of records returned in calls.",
				Computed:            true,
			},
			"truncated": schema.BoolAttribute{
				MarkdownDescription: "True when `max_records` cut off pagination before the backend was exhausted. Re-run with a higher cap or narrower filter to see all matches.",
				Computed:            true,
			},
			"calls": schema.ListNestedAttribute{
				MarkdownDescription: "Matching tool-call records in the API cursor order (newest first).",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"app_id":          schema.StringAttribute{Computed: true, Sensitive: false, MarkdownDescription: "App UUID when invoked by an app."},
						"app_name":        schema.StringAttribute{Computed: true, Sensitive: false, MarkdownDescription: "App display name."},
						"oauth_client_id": schema.StringAttribute{Computed: true, Sensitive: false, MarkdownDescription: "OAuth client ID."},
						"owner_type":      schema.StringAttribute{Computed: true, Sensitive: false, MarkdownDescription: "Owning principal type."},
						"run_id":          schema.StringAttribute{Computed: true, Sensitive: false, MarkdownDescription: "Agent run ID."},
						"source":          schema.StringAttribute{Computed: true, Sensitive: false, MarkdownDescription: "Invocation source."},
						"tool_call_json":  schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Complete tool-call payload as JSON, including extension fields."},
						"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Call UUID."},
						"agent_id":        schema.StringAttribute{Computed: true, MarkdownDescription: "UUID of the agent that initiated the call. Null for calls outside an agent context."},
						"mcp_server_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Display name of the MCP server that served the call."},
						"method":          schema.StringAttribute{Computed: true, MarkdownDescription: "MCP method name (e.g. `tools/call`)."},
						"tool_name":       schema.StringAttribute{Computed: true, MarkdownDescription: "Name of the tool invoked. Null if the call wasn't a tool/call."},
						"tool_call_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "Provider-generated correlation ID inside `toolCall`. Null if absent."},
						"arguments":       schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Tool-call arguments encoded as a JSON string. Use `jsondecode(call.arguments)` to introspect. Null if absent."},
						"result":          schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Tool result encoded as a JSON string (the wire shape is polymorphic — use `jsondecode`). Null if absent."},
						"auth_method":     schema.StringAttribute{Computed: true, MarkdownDescription: "How the agent authenticated the call (`oauth`, `user_token`, `org_token`, `team_token`, `external_idp`, `session`). Null if absent."},
						"user_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "UUID of the user the call ran on behalf of. Null for org-wide calls."},
						"user_name":       schema.StringAttribute{Computed: true, MarkdownDescription: "Display name of the user. Null when `user_id` is null."},
						"created_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 timestamp of when the call was recorded."},
					},
				},
			},
		},
	}
}

func (d *McpToolCallsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *McpToolCallsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data McpToolCallsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &client.GetMcpToolCallsParams{}

	if !data.AgentID.IsNull() && !data.AgentID.IsUnknown() {
		u, err := uuid.Parse(data.AgentID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid agent_id", err.Error())
			return
		}
		params.AgentId = &u
	}
	if !data.StartDate.IsNull() && !data.StartDate.IsUnknown() {
		t, err := time.Parse(time.RFC3339, data.StartDate.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid start_date", fmt.Sprintf("expected RFC 3339, got %q: %s", data.StartDate.ValueString(), err))
			return
		}
		params.StartDate = &t
	}
	if !data.EndDate.IsNull() && !data.EndDate.IsUnknown() {
		t, err := time.Parse(time.RFC3339, data.EndDate.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid end_date", fmt.Sprintf("expected RFC 3339, got %q: %s", data.EndDate.ValueString(), err))
			return
		}
		params.EndDate = &t
	}
	if !data.ServerName.IsNull() && !data.ServerName.IsUnknown() {
		name := data.ServerName.ValueString()
		params.McpServerName = &name
	}

	maxRecords := defaultMcpToolCallsMaxRecords
	if !data.MaxRecords.IsNull() && !data.MaxRecords.IsUnknown() {
		maxRecords = data.MaxRecords.ValueInt64()
		if maxRecords <= 0 {
			resp.Diagnostics.AddError("Invalid max_records", "max_records must be > 0")
			return
		}
	}

	limit := 100
	params.Limit = &limit

	var collected []attr.Value
	seenCursors := map[string]bool{}
	truncated := false

	for {
		callsResp, err := d.client.GetMcpToolCallsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read MCP tool calls: %s", err))
			return
		}
		if callsResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d", callsResp.StatusCode()),
			)
			return
		}

		for i := range callsResp.JSON200.Data {
			if int64(len(collected)) >= maxRecords {
				truncated = true
				break
			}
			c := &callsResp.JSON200.Data[i]
			obj, diags := flattenMcpToolCall(c)
			resp.Diagnostics.Append(diags...)
			if diags.HasError() {
				return
			}
			collected = append(collected, obj)
		}

		if truncated || !callsResp.JSON200.Pagination.HasNext {
			break
		}
		if int64(len(collected)) >= maxRecords {
			truncated = true
			break
		}
		next := callsResp.JSON200.Pagination.NextCursor
		if next == nil || *next == "" || seenCursors[*next] {
			resp.Diagnostics.AddError("Invalid API Pagination", "The backend indicated another page without a new cursor.")
			return
		}
		seenCursors[*next] = true
		params.Cursor = next
	}

	listValue, diags := types.ListValue(mcpToolCallObjectType, collected)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	data.Calls = listValue
	data.ReturnedCount = types.Int64Value(int64(len(collected)))
	data.Truncated = types.BoolValue(truncated)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// flattenMcpToolCall projects one wire record onto the data-source schema.
type mcpToolCallWire struct {
	AppID         *string         `json:"appId"`
	AppName       *string         `json:"appName"`
	OAuthClientID *string         `json:"oauthClientId"`
	OwnerType     string          `json:"ownerType"`
	RunID         *string         `json:"runId"`
	Source        *string         `json:"source"`
	ToolCall      json.RawMessage `json:"toolCall"`

	AgentId       *openapi_types.UUID                      `json:"agentId"`
	AuthMethod    *client.GetMcpToolCalls200DataAuthMethod `json:"authMethod"`
	CreatedAt     time.Time                                `json:"createdAt"`
	Id            openapi_types.UUID                       `json:"id"`
	McpServerName string                                   `json:"mcpServerName"`
	Method        string                                   `json:"method"`
	ToolResult    interface{}                              `json:"toolResult"`
	UserId        *string                                  `json:"userId"`
	UserName      *string                                  `json:"userName"`
}

func flattenMcpToolCall(raw any) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	encoded, err := json.Marshal(raw)
	if err != nil {
		diags.AddError("Invalid Tool Call", err.Error())
		return nil, diags
	}
	var c mcpToolCallWire
	if err := json.Unmarshal(encoded, &c); err != nil {
		diags.AddError("Invalid Tool Call", err.Error())
		return nil, diags
	}

	// helper closures for nullable fields
	strPtrOrNull := func(s *string) types.String {
		if s == nil {
			return types.StringNull()
		}
		return types.StringValue(*s)
	}
	jsonOrNull := func(v interface{}) types.String {
		if v == nil {
			return types.StringNull()
		}
		b, err := json.Marshal(v)
		if err != nil {
			return types.StringNull()
		}
		return types.StringValue(string(b))
	}

	agentID := types.StringNull()
	if c.AgentId != nil {
		agentID = types.StringValue(c.AgentId.String())
	}
	authMethod := types.StringNull()
	if c.AuthMethod != nil {
		authMethod = types.StringValue(string(*c.AuthMethod))
	}

	toolName := types.StringNull()
	toolCallID := types.StringNull()
	arguments := types.StringNull()
	toolCallJSON := types.StringNull()
	if len(c.ToolCall) > 0 && string(c.ToolCall) != "null" {
		toolCallJSON = types.StringValue(string(c.ToolCall))
		var call struct {
			Name      *string     `json:"name"`
			ID        *string     `json:"id"`
			Arguments interface{} `json:"arguments"`
		}
		if json.Unmarshal(c.ToolCall, &call) == nil {
			toolName = strPtrOrNull(call.Name)
			toolCallID = strPtrOrNull(call.ID)
			arguments = jsonOrNull(call.Arguments)
		}
	}

	return types.ObjectValue(mcpToolCallObjectType.AttrTypes, map[string]attr.Value{
		"app_id":          strPtrOrNull(c.AppID),
		"app_name":        strPtrOrNull(c.AppName),
		"oauth_client_id": strPtrOrNull(c.OAuthClientID),
		"owner_type":      types.StringValue(c.OwnerType),
		"run_id":          strPtrOrNull(c.RunID),
		"source":          strPtrOrNull(c.Source),
		"tool_call_json":  toolCallJSON,
		"id":              types.StringValue(c.Id.String()),
		"agent_id":        agentID,
		"mcp_server_name": types.StringValue(c.McpServerName),
		"method":          types.StringValue(c.Method),
		"tool_name":       toolName,
		"tool_call_id":    toolCallID,
		"arguments":       arguments,
		"result":          jsonOrNull(c.ToolResult),
		"auth_method":     authMethod,
		"user_id":         strPtrOrNull(c.UserId),
		"user_name":       strPtrOrNull(c.UserName),
		"created_at":      types.StringValue(c.CreatedAt.Format(time.RFC3339)),
	})
}
