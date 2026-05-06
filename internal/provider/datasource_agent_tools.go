package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &AgentToolsDataSource{}

func NewAgentToolsDataSource() datasource.DataSource {
	return &AgentToolsDataSource{}
}

type AgentToolsDataSource struct {
	client *client.ClientWithResponses
}

type AgentToolsDataSourceModel struct {
	AgentID types.String `tfsdk:"agent_id"`
	Tools   types.List   `tfsdk:"tools"`
}

// agentToolListItemObjectType is the per-element shape of the plural
// `tools` list. Mirrors the wire shape of GetAllAgentTools but kept flat
// for HCL ergonomics: assignment-level fields (assignment_id,
// credential_resolution_mode, mcp_server_id) sit alongside tool-level
// fields (tool_id, name, description) so a single `for_each` block has
// everything it needs to drive policy + assignment resources.
var agentToolListItemObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"assignment_id":              types.StringType,
	"tool_id":                    types.StringType,
	"name":                       types.StringType,
	"description":                types.StringType,
	"mcp_server_id":              types.StringType,
	"credential_resolution_mode": types.StringType,
	"created_at":                 types.StringType,
}}

func (d *AgentToolsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_tools"
}

func (d *AgentToolsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every tool currently assigned to an agent.",

		Attributes: map[string]schema.Attribute{
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "Agent UUID. Pulls the assignment list from `/api/agent-tools?agentId=<id>`.",
				Required:            true,
			},
			"tools": schema.ListNestedAttribute{
				MarkdownDescription: "Every tool currently assigned to the agent, including assignment-level metadata.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"assignment_id": schema.StringAttribute{
							MarkdownDescription: "Agent-tool assignment composite UUID. **Not** what `archestra_tool_invocation_policy.tool_id` expects — see `tool_id` below for that.",
							Computed:            true,
						},
						"tool_id": schema.StringAttribute{
							MarkdownDescription: "Bare tool UUID. Pass this as `tool_id` on `archestra_tool_invocation_policy` / `archestra_trusted_data_policy`.",
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
						"mcp_server_id": schema.StringAttribute{
							MarkdownDescription: "MCP server installation UUID this assignment is bound to. Null for tools assigned without a specific install (e.g. delegated tools).",
							Computed:            true,
						},
						"credential_resolution_mode": schema.StringAttribute{
							MarkdownDescription: "How the agent resolves the credential it uses to call this tool: `static`, `dynamic`, or `enterprise_managed`. See the matching field on `archestra_agent_tool`.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "RFC 3339 timestamp of when the assignment was made. Useful as a stable sort key.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *AgentToolsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *AgentToolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentToolsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", fmt.Sprintf("Could not parse agent_id as UUID: %s", err))
		return
	}

	limit := 100
	offset := 0
	var collected []attr.Value

	for {
		toolsResp, err := d.client.GetAllAgentToolsWithResponse(ctx, &client.GetAllAgentToolsParams{
			AgentId: &agentUUID,
			Limit:   &limit,
			Offset:  &offset,
		})
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent tools: %s", err))
			return
		}
		if toolsResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d", toolsResp.StatusCode()),
			)
			return
		}

		for i := range toolsResp.JSON200.Data {
			at := &toolsResp.JSON200.Data[i]

			desc := types.StringNull()
			if at.Tool.Description != nil {
				desc = types.StringValue(*at.Tool.Description)
			}
			mcpServerID := types.StringNull()
			if at.McpServerId != nil {
				mcpServerID = types.StringValue(at.McpServerId.String())
			}

			obj, diags := types.ObjectValue(agentToolListItemObjectType.AttrTypes, map[string]attr.Value{
				"assignment_id":              types.StringValue(at.Id.String()),
				"tool_id":                    types.StringValue(at.Tool.Id),
				"name":                       types.StringValue(at.Tool.Name),
				"description":                desc,
				"mcp_server_id":              mcpServerID,
				"credential_resolution_mode": types.StringValue(string(at.CredentialResolutionMode)),
				"created_at":                 types.StringValue(at.CreatedAt.Format(time.RFC3339)),
			})
			resp.Diagnostics.Append(diags...)
			if diags.HasError() {
				return
			}
			collected = append(collected, obj)
		}

		if !toolsResp.JSON200.Pagination.HasNext {
			break
		}
		offset += limit
	}

	listValue, diags := types.ListValue(agentToolListItemObjectType, collected)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	data.Tools = listValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
