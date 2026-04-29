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
	AgentID   types.String `tfsdk:"agent_id"`
	StartDate types.String `tfsdk:"start_date"`
	EndDate   types.String `tfsdk:"end_date"`
	Search    types.String `tfsdk:"search"`
	Calls     types.List   `tfsdk:"calls"`
	Total     types.Int64  `tfsdk:"total"`
}

// mcpToolCallObjectType is the per-element shape of `calls`. Mirrors the
// wire as closely as is HCL-friendly: arguments and result come back as
// JSON strings (use `jsondecode` to introspect), the rest are flat.
var mcpToolCallObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
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
		MarkdownDescription: "Queries the MCP tool-call audit log. Useful for compliance attestations, " +
			"alarm thresholds keyed off `length(...calls) > N`, or driving downstream resources off recent activity " +
			"(e.g. only assigning a tool to an agent that called it in the last 24h).\n\n" +
			"```hcl\n" +
			"data \"archestra_mcp_tool_calls\" \"recent_failures\" {\n" +
			"  agent_id   = archestra_agent.support.id\n" +
			"  start_date = timeadd(timestamp(), \"-24h\")\n" +
			"  search     = \"error\"\n" +
			"}\n\n" +
			"output \"failure_count_24h\" {\n" +
			"  value = data.archestra_mcp_tool_calls.recent_failures.total\n" +
			"}\n" +
			"```\n\n" +
			"~> **Pagination is exhaustive.** This data source iterates the backend's paginated `/api/mcp-tool-calls` endpoint until exhausted (`limit=100` per page) and surfaces every match. Filter narrowly with `start_date` / `end_date` / `search` to keep state size manageable.",

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
			"search": schema.StringAttribute{
				MarkdownDescription: "Optional. Free-text, case-insensitive substring matched against MCP server name, tool name, and arguments JSON.",
				Optional:            true,
			},
			"total": schema.Int64Attribute{
				MarkdownDescription: "Total number of calls matching the filter (across all pages, before any local truncation).",
				Computed:            true,
			},
			"calls": schema.ListNestedAttribute{
				MarkdownDescription: "Matching tool-call records, oldest-first within each page (server's default sort: `createdAt desc` per page, but pages are aggregated in fetch order).",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Call UUID."},
						"agent_id":        schema.StringAttribute{Computed: true, MarkdownDescription: "UUID of the agent that initiated the call. Null for calls outside an agent context."},
						"mcp_server_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Display name of the MCP server that served the call."},
						"method":          schema.StringAttribute{Computed: true, MarkdownDescription: "MCP method name (e.g. `tools/call`)."},
						"tool_name":       schema.StringAttribute{Computed: true, MarkdownDescription: "Name of the tool invoked. Null if the call wasn't a tool/call."},
						"tool_call_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "Provider-generated correlation ID inside `toolCall`. Null if absent."},
						"arguments":       schema.StringAttribute{Computed: true, MarkdownDescription: "Tool-call arguments encoded as a JSON string. Use `jsondecode(call.arguments)` to introspect. Null if absent."},
						"result":          schema.StringAttribute{Computed: true, MarkdownDescription: "Tool result encoded as a JSON string (the wire shape is polymorphic — use `jsondecode`). Null if absent."},
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
	if !data.Search.IsNull() && !data.Search.IsUnknown() {
		s := data.Search.ValueString()
		params.Search = &s
	}

	limit := 100
	offset := 0
	params.Limit = &limit
	params.Offset = &offset

	var collected []attr.Value
	var total int64

	for {
		callsResp, err := d.client.GetMcpToolCallsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read MCP tool calls: %s", err))
			return
		}
		if callsResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d: %s", callsResp.StatusCode(), string(callsResp.Body)),
			)
			return
		}
		total = int64(callsResp.JSON200.Pagination.Total)

		for i := range callsResp.JSON200.Data {
			c := &callsResp.JSON200.Data[i]
			obj, diags := flattenMcpToolCall(c)
			resp.Diagnostics.Append(diags...)
			if diags.HasError() {
				return
			}
			collected = append(collected, obj)
		}

		if !callsResp.JSON200.Pagination.HasNext {
			break
		}
		offset += limit
		params.Offset = &offset
	}

	listValue, diags := types.ListValue(mcpToolCallObjectType, collected)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	data.Calls = listValue
	data.Total = types.Int64Value(total)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// flattenMcpToolCall projects one wire record onto the data-source schema.
func flattenMcpToolCall(c *struct {
	AgentId       *openapi_types.UUID                      `json:"agentId"`
	AuthMethod    *client.GetMcpToolCalls200DataAuthMethod `json:"authMethod"`
	CreatedAt     time.Time                                `json:"createdAt"`
	Id            openapi_types.UUID                       `json:"id"`
	McpServerName string                                   `json:"mcpServerName"`
	Method        string                                   `json:"method"`
	ToolCall      *struct {
		Arguments map[string]interface{} `json:"arguments"`
		Id        string                 `json:"id"`
		Name      string                 `json:"name"`
	} `json:"toolCall"`
	ToolResult interface{} `json:"toolResult"`
	UserId     *string     `json:"userId"`
	UserName   *string     `json:"userName"`
}) (attr.Value, diag.Diagnostics) {
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
	if c.ToolCall != nil {
		toolName = types.StringValue(c.ToolCall.Name)
		toolCallID = types.StringValue(c.ToolCall.Id)
		arguments = jsonOrNull(c.ToolCall.Arguments)
	}

	return types.ObjectValue(mcpToolCallObjectType.AttrTypes, map[string]attr.Value{
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
