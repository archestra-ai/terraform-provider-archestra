package provider

import (
	"context"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func NewMcpRegistryEntryDataSource() datasource.DataSource {
	return &namedLookupDataSource{spec: namedLookupSpec{
		typeName:    "mcp_registry_entry",
		noun:        "MCP Registry Entry",
		description: "Finds an MCP registry entry by its exact name, for example to install one added in the UI with `archestra_mcp_server`.",
		key:         namedLookupField{tf: "name", json: "name", desc: "Exact name of the registry entry."},
		fields: []namedLookupField{
			{tf: "description", json: "description", desc: "Entry description."},
			{tf: "server_type", json: "serverType", desc: "How the server runs: `local`, `remote` or `builtin`."},
			{tf: "scope", json: "scope", desc: "Who the entry is shared with."},
			{tf: "tool_prefix", json: "toolPrefix", desc: "Prefix of the tool names the server's tools get."},
		},
		list: func(ctx context.Context, c *client.ClientWithResponses, _ string) ([]map[string]any, error) {
			r, err := c.GetInternalMcpCatalogWithResponse(ctx, &client.GetInternalMcpCatalogParams{})
			if err != nil {
				return nil, err
			}
			items, _, err := listItems(r.StatusCode(), r.Body)
			return items, err
		},
	}}
}
