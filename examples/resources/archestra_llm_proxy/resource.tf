resource "archestra_llm_proxy" "shared" {
  name        = "shared-openai"
  description = "Shared org-wide proxy for OpenAI traffic."

  passthrough_headers = ["x-correlation-id", "x-tenant-id"]

  labels = [
    { key = "team", value = "platform" }
  ]
}

# Same proxy, but requiring inbound JWTs validated against an identity provider.
resource "archestra_llm_proxy" "authenticated" {
  name                 = "secure-openai"
  description          = "OpenAI proxy behind JWT auth."
  identity_provider_id = archestra_identity_provider.oidc.id
}
