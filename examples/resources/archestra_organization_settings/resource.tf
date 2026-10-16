# --- External references this example assumes exist in your module ---
#
#   archestra_model_provider.inline   (used by default_llm_api_key_id, embedding_chat_api_key_id)
#   archestra_model_provider.cohere   (used by reranker_chat_api_key_id)
#   archestra_agent.support                 (used by default_agent_id)
#   archestra_environment.production        (default environment)
#   archestra_team.engineering              (default agent access)
#
# See examples/resources/archestra_model_provider/ and
# examples/resources/archestra_agent/ for matching declarations.

# Singleton resource — exactly one row exists per organization. `terraform
# destroy` only drops the local state; the backend keeps whatever values were
# last applied.
resource "archestra_organization_settings" "main" {
  # --- Appearance ---
  font        = "inter"
  color_theme = "modern-minimal"
  app_name    = "Acme Copilot"
  footer_text = "© 2026 Acme Inc."

  # Inline base64 logos. Use `filebase64()` so updates churn cleanly.
  # Drop your own PNGs into `assets/` alongside this file and uncomment;
  # leaving them out keeps the platform's defaults.
  # logo      = filebase64("${path.module}/assets/logo.png")
  # logo_dark = filebase64("${path.module}/assets/logo-dark.png")
  # favicon   = filebase64("${path.module}/assets/favicon.png")

  # --- Chat UX ---
  chat_placeholders         = ["What can I help with?", "Ask anything…"]
  animate_chat_placeholders = true
  chat_links = [
    { label = "Docs", url = "https://docs.acme.com" },
    { label = "Status", url = "https://status.acme.com" },
  ]
  chat_error_support_message = "Something went wrong. Email support@acme.com."
  slim_chat_error_ui         = false
  allow_chat_file_uploads    = true

  default_llm_api_key_id = archestra_model_provider.inline.id
  default_agent_id       = archestra_agent.support.id

  # --- Knowledge / RAG ---
  # WARNING: embedding_* fields are write-once. Pick carefully — changing them
  # later requires dropping the embedding config server-side first.
  embedding_model           = "text-embedding-3-small"
  embedding_chat_api_key_id = archestra_model_provider.inline.id
  reranker_model            = "rerank-english-v3.0"
  reranker_chat_api_key_id  = archestra_model_provider.cohere.id


  # Settings owned by this organization, rather than separate binding resources.
  environment_defaults = {
    agents       = archestra_environment.production.id
    mcp_gateways = archestra_environment.production.id
  }
  default_user_limits = {
    default = {
      limit_value      = 10
      models           = [] # All models
      cleanup_interval = "calendar_month"
    }
  }
  default_permissions = {
    agents = [{
      subject_type = "team"
      subject_id   = archestra_team.engineering.id
      actions      = ["read"]
    }]
  }

  # One-way flag — once set to true on the backend, attempts to flip back
  # to false are rejected. Leave commented until you've verified the org.
  # onboarding_complete = true
}
