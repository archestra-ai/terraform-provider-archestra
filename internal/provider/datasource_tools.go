package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ToolsDataSource{}

func NewToolsDataSource() datasource.DataSource {
	return &ToolsDataSource{}
}

// ToolsDataSource maps many tool names to IDs in a few paged calls, where
// `archestra_tool` makes one call per name.
type ToolsDataSource struct {
	client *client.ClientWithResponses
}

type ToolsDataSourceModel struct {
	NamePrefix   types.String `tfsdk:"name_prefix"`
	ToolIDByName types.Map    `tfsdk:"tool_id_by_name"`
}

func (d *ToolsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tools"
}

func (d *ToolsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the tools whose names start with a prefix, such as the built-in `archestra__` tools, as a map from name to ID. " +
			"For the tools of one MCP server installation, use `tool_id_by_name` of `archestra_mcp_server`.",
		Attributes: map[string]schema.Attribute{
			"name_prefix": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Prefix the tool names start with, e.g. `archestra__`.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"tool_id_by_name": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Tool UUID by full tool name. A name that several tools share is left out.",
			},
		},
	}
}

func (d *ToolsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	apiClient, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = apiClient
}

func (d *ToolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ToolsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	prefix := data.NamePrefix.ValueString()

	const pageLimit = 100
	limit, search := pageLimit, prefix
	var tools []toolLookup
	for offset := 0; ; offset += pageLimit {
		page, err := d.client.GetToolsWithAssignmentsWithResponse(ctx, &client.GetToolsWithAssignmentsParams{Search: &search, Limit: &limit, Offset: &offset})
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list tools: %s", err))
			return
		}
		if page.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Listing tools: expected 200 OK, got status %d", page.StatusCode()))
			return
		}
		for _, tool := range page.JSON200.Data {
			tools = append(tools, toolLookup{ID: tool.Id, Name: tool.Name})
		}
		if !page.JSON200.Pagination.HasNext {
			break
		}
		if len(page.JSON200.Data) == 0 {
			resp.Diagnostics.AddError("Unexpected API Response", "The tool listing returned an empty page with more results.")
			return
		}
	}

	byName, diags := toolIDsWithPrefix(tools, prefix)
	resp.Diagnostics.Append(diags...)
	data.ToolIDByName = byName
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// toolIDsWithPrefix keeps the tools whose names start with prefix (the API's
// search matches anywhere in the name) and leaves out names several tools
// share, so a name never maps to the wrong tool.
func toolIDsWithPrefix(tools []toolLookup, prefix string) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	ids := map[string]string{}
	shared := map[string]bool{}
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, prefix) {
			continue
		}
		if existing, seen := ids[tool.Name]; seen && existing != tool.ID {
			shared[tool.Name] = true
			continue
		}
		ids[tool.Name] = tool.ID
	}
	entries := make(map[string]attr.Value, len(ids))
	for name, id := range ids {
		if shared[name] {
			diags.AddWarning("Duplicate tool name", fmt.Sprintf("Several tools are named %q, so it is left out of `tool_id_by_name`; look it up with `archestra_tool` and `mcp_server_id`.", name))
			continue
		}
		entries[name] = types.StringValue(id)
	}
	out, d := types.MapValue(types.StringType, entries)
	diags.Append(d...)
	return out, diags
}
