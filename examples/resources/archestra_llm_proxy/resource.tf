# Externals (declare elsewhere): archestra_llm_provider_api_key.inline, archestra_team.engineering, archestra_identity_provider.oidc.

# NOTE: `llm_model` is the model_id from the upstream LLM API. The platform
# auto-discovers models from `archestra_llm_provider_api_key`, so the key
# resource for the chosen provider must exist. `archestra_llm_model` is
# only for overriding pricing / per-model settings on a discovered model.

# Org-wide LLM proxy — fronts an upstream model so apps can hit Archestra
# instead of talking to OpenAI directly. Headers in `passthrough_headers`
# survive the hop, which is how downstream services see request IDs / tenants.
resource "archestra_llm_proxy" "shared" {
  name        = "shared-openai"
  description = "Shared org-wide proxy for OpenAI traffic."
  llm_model   = "gpt-4o"

  passthrough_headers = ["x-correlation-id", "x-tenant-id"]

  labels = [
    { key = "team", value = "platform" },
    { key = "environment", value = "production" },
  ]
}

# Authenticated proxy — requires inbound JWTs validated against the configured
# identity provider. Useful for exposing the proxy outside the cluster.
resource "archestra_llm_proxy" "authenticated" {
  name                 = "secure-openai"
  description          = "OpenAI proxy behind JWT auth."
  llm_model            = "gpt-4o"
  identity_provider_id = archestra_identity_provider.oidc.id

  # Use a dedicated provider key so this proxy's spend is attributable.
  llm_api_key_id = archestra_llm_provider_api_key.inline.id
}

# Team-scoped proxy — only visible inside the listed teams.
resource "archestra_llm_proxy" "engineering" {
  name        = "engineering-claude"
  description = "Claude proxy for the engineering org"
  llm_model   = "claude-sonnet-4-5"
  scope       = "team"
  teams       = [archestra_team.engineering.id]
}
