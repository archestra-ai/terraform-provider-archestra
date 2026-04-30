terraform {
  required_providers {
    archestra = {
      source  = "archestra-ai/archestra"
      version = "~> 0.6.0"
    }
  }
}

provider "archestra" {
  # base_url + api_key are read from ARCHESTRA_BASE_URL / ARCHESTRA_API_KEY.
  # Don't commit keys to source.
}

resource "archestra_organization_settings" "main" {
  app_name           = var.app_name
  footer_text        = var.footer_text
  global_tool_policy = "restrictive"

  # Chat UX
  chat_placeholders         = ["What can I help with?", "Ask anything…"]
  animate_chat_placeholders = true
  chat_links = [
    { label = "Docs", url = "https://docs.demo.example.com" },
    { label = "Status", url = "https://status.demo.example.com" },
  ]
  chat_error_support_message = "Something went wrong. Email support@demo.example.com."
  slim_chat_error_ui         = false
  allow_chat_file_uploads    = true

  # Compression
  compression_scope            = "team"
  convert_tool_results_to_toon = false

  # Cost / limit cleanup
  limit_cleanup_interval = "24h"

  # MCP OAuth tuning
  mcp_oauth_access_token_lifetime_seconds = 3600

  # Auth UX
  show_two_factor = true

  # Default LLM + agent — cross-refs to our existing resources. Without these
  # the platform falls back to whichever provider key is is_organization_default.
  default_llm_provider   = "ollama"
  default_llm_model      = "llama3"
  default_llm_api_key_id = archestra_llm_provider_api_key.ollama_vault.id
  default_agent_id       = archestra_agent.support.id

  # NOTE: embedding_* and reranker_* are deliberately omitted from this demo.
  # The platform treats them as write-once — changing the embedding model after
  # any KB has been indexed requires server-side cleanup. Set them in your own
  # module on initial bring-up only.
}
