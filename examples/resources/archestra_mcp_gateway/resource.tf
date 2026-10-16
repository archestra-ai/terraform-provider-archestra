# Externals (declare elsewhere): archestra_team.engineering, archestra_identity_provider.oidc.

# Org-wide MCP gateway — clients (Claude Desktop, Cursor, etc.) point at
# this endpoint and Archestra federates the tools from every install behind it.
resource "archestra_mcp_gateway" "default" {
  # Gateway settings and assignments share the gateway's lifecycle.
  subagent_ids           = []
  remote_agent_ids       = []
  tools                  = []
  excluded_tool_ids      = []
  excluded_subagent_ids  = []
  excluded_connector_ids = []
  skills                 = { mode = "manual", ids = [], excluded_ids = [] }

  name        = "production-mcp"
  description = "Default MCP gateway exposed to org clients."

  passthrough_headers = ["x-correlation-id"]

  labels = [
    { key = "environment", value = "production" },
  ]
}

# JWT-authenticated gateway — every inbound request must carry a token signed
# by the configured identity provider. Pair with `archestra_identity_provider`.
resource "archestra_mcp_gateway" "authenticated" {
  name                 = "secure-mcp"
  description          = "MCP gateway behind JWT auth."
  identity_provider_id = archestra_identity_provider.oidc.id
}

# Grant team access with archestra_resource_permission_policy. Combined
# with the gateway tools attribute, this is how teams get their own tool surface.
resource "archestra_mcp_gateway" "engineering" {
  name        = "engineering-mcp"
  description = "MCP gateway scoped to engineering"

  # Knowledge bases / connectors are surfaced as MCP resources on the gateway.
  knowledge_base_ids = []
  connector_ids      = []
}
