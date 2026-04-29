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
  logo      = filebase64("${path.module}/assets/logo.png")
  logo_dark = filebase64("${path.module}/assets/logo-dark.png")
  favicon   = filebase64("${path.module}/assets/favicon.png")

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

  # --- LLM defaults ---
  default_llm_provider   = "openai"
  default_llm_model      = "gpt-4o"
  default_llm_api_key_id = archestra_llm_provider_api_key.inline.id
  default_agent_id       = archestra_agent.support.id

  # --- Knowledge / RAG ---
  # WARNING: embedding_* fields are write-once. Pick carefully — changing them
  # later requires dropping the embedding config server-side first.
  embedding_model           = "text-embedding-3-small"
  embedding_chat_api_key_id = archestra_llm_provider_api_key.inline.id
  reranker_model            = "rerank-english-v3.0"
  reranker_chat_api_key_id  = archestra_llm_provider_api_key.cohere.id

  # --- Tool / context security ---
  global_tool_policy = "restrictive" # or "permissive"

  # --- Compression / TOON ---
  compression_scope            = "organization" # one of: organization, team
  convert_tool_results_to_toon = true

  # --- Cost / limit cleanup ---
  limit_cleanup_interval = "24h" # 1h | 12h | 24h | 1w | 1m

  # --- MCP OAuth tuning ---
  mcp_oauth_access_token_lifetime_seconds = 3600

  # --- Auth UX ---
  show_two_factor = true

  # One-way flag — once set to true on the backend, attempts to flip back
  # to false are rejected. Leave commented until you've verified the org.
  # onboarding_complete = true
}
