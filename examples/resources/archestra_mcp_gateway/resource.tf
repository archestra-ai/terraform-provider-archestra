resource "archestra_mcp_gateway" "default" {
  name        = "production-mcp"
  description = "Default MCP gateway exposed to org clients."

  passthrough_headers = ["x-correlation-id"]

  labels = [
    { key = "environment", value = "production" }
  ]
}

# Same gateway, but requiring inbound JWTs validated against an identity provider.
resource "archestra_mcp_gateway" "authenticated" {
  name                 = "secure-mcp"
  description          = "MCP gateway behind JWT auth."
  identity_provider_id = archestra_identity_provider.oidc.id
}
