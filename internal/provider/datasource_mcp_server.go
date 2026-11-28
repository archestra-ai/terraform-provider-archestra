package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &McpServerDataSource{}

func NewMcpServerDataSource() datasource.DataSource { return &McpServerDataSource{} }

// McpServerDataSource finds an MCP server installation that exists already,
// such as a person's connected account, so a tool assignment can pin it
// (`mcp_server_id`) without a pasted UUID.
type McpServerDataSource struct {
	client *client.ClientWithResponses
}

type McpServerDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	CatalogID    types.String `tfsdk:"catalog_id"`
	CatalogName  types.String `tfsdk:"catalog_name"`
	OwnerEmail   types.String `tfsdk:"owner_email"`
	Scope        types.String `tfsdk:"scope"`
	TeamID       types.String `tfsdk:"team_id"`
	ToolIDByName types.Map    `tfsdk:"tool_id_by_name"`
}

func (d *McpServerDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server"
}

func (d *McpServerDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds an installed MCP server that exists already. Set at least one of `name`, `catalog_id`, `catalog_name` and `owner_email`; every one that is set must match, and exactly one installation must remain. Use `id` as the `mcp_server_id` of a tool assignment. People's own installations of one registry entry share a name, so select one with `owner_email`.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, MarkdownDescription: "ID of the installation."},
			"name": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Exact name of the installation."},
			"tool_id_by_name": schema.MapAttribute{
				Computed: true, ElementType: types.StringType,
				MarkdownDescription: "Tool UUID by full tool name (`<server>__<short>`), for tool assignments.",
			},
			"catalog_id":   schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "ID of the MCP registry entry the server was installed from."},
			"catalog_name": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Exact name of the MCP registry entry the server was installed from."},
			"owner_email":  schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Email address of the person who owns the installation."},
			"scope":        schema.StringAttribute{Computed: true, MarkdownDescription: "Who the installation is shared with."},
			"team_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Team the installation belongs to, for a team installation."},
		},
	}
}

func (d *McpServerDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = dataSourceClient(req, resp)
}

func (d *McpServerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data McpServerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name, catalogID, catalogName, ownerEmail := data.Name.ValueString(), data.CatalogID.ValueString(), data.CatalogName.ValueString(), data.OwnerEmail.ValueString()
	if name == "" && catalogID == "" && catalogName == "" && ownerEmail == "" {
		resp.Diagnostics.AddError("Missing Lookup", "Set at least one of name, catalog_id, catalog_name and owner_email.")
		return
	}

	params := &client.GetMcpServersParams{}
	if catalogID != "" {
		params.CatalogId = &catalogID
	}
	servers, err := d.client.GetMcpServersWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list MCP servers: %s", err))
		return
	}
	if servers.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", servers.StatusCode()))
		return
	}

	matches := func(wanted string, actual *string) bool {
		return wanted == "" || (actual != nil && *actual == wanted)
	}
	found := -1
	for index, server := range *servers.JSON200 {
		serverCatalogID := server.CatalogId.String()
		if !matches(name, &server.Name) || !matches(catalogID, &serverCatalogID) || !matches(catalogName, server.CatalogName) || !matches(ownerEmail, server.OwnerEmail) {
			continue
		}
		if found >= 0 {
			resp.Diagnostics.AddError("Ambiguous MCP Server", "Several installations match. Narrow the lookup, for example with owner_email.")
			return
		}
		found = index
	}
	if found < 0 {
		resp.Diagnostics.AddError("MCP Server Not Found", "No MCP server installation visible to this API key matches the lookup.")
		return
	}

	server := (*servers.JSON200)[found]
	data.ID = types.StringValue(server.Id.String())
	data.Name = types.StringValue(server.Name)
	data.CatalogID = types.StringValue(server.CatalogId.String())
	data.CatalogName = types.StringPointerValue(server.CatalogName)
	data.OwnerEmail = types.StringPointerValue(server.OwnerEmail)
	data.Scope = types.StringValue(string(server.Scope))
	data.TeamID = types.StringPointerValue(server.TeamId)

	// One call returns every tool of the installation, so a configuration
	// maps names to IDs without a lookup per tool.
	tools, err := d.client.GetMcpServerToolsWithResponse(ctx, server.Id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list the MCP server's tools: %s", err))
		return
	}
	if tools.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Listing the MCP server's tools: expected 200 OK, got status %d", tools.StatusCode()))
		return
	}
	_, toolIDs, diags := projectMcpServerTools(*tools.JSON200)
	resp.Diagnostics.Append(diags...)
	data.ToolIDByName = toolIDs
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
