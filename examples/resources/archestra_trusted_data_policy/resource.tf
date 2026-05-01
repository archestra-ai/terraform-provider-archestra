resource "archestra_mcp_registry_catalog_item" "fetch" {
  name        = "fetch"
  description = "URL-fetching MCP server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-fetch"]
  }
}

resource "archestra_mcp_server_installation" "fetch" {
  name       = "fetch"
  catalog_id = archestra_mcp_registry_catalog_item.fetch.id
}

resource "archestra_trusted_data_policy" "trust_company_api" {
  tool_id     = archestra_mcp_server_installation.fetch.tool_id_by_name["${archestra_mcp_registry_catalog_item.fetch.name}__fetch"]
  description = "Mark data from company API as trusted"
  conditions = [
    { key = "url", operator = "contains", value = "api.company.com" },
  ]
  action = "mark_as_trusted"
}
