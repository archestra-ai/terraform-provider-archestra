# Org-wide MCP gateway — clients (Claude Desktop, Cursor, etc.) point at
# this endpoint and Archestra federates the tools from every install behind it.
resource "archestra_mcp_gateway" "default" {
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

# Team-scoped gateway — only members of the listed teams see it. Combined
# with `archestra_agent_tool`, this is how teams get their own tool surface.
resource "archestra_mcp_gateway" "engineering" {
  name        = "engineering-mcp"
  description = "MCP gateway scoped to engineering"
  scope       = "team"
  teams       = [archestra_team.engineering.id]

  # Knowledge bases / connectors are surfaced as MCP resources on the gateway.
  knowledge_base_ids = []
  connector_ids      = []
}
