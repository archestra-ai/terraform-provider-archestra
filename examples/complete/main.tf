terraform {
  required_providers {
    archestra = {
      source  = "archestra-ai/archestra"
      version = "~> 3.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.6.3"
    }
  }
}

# Per-apply random suffix for IdP provider_ids. Ensures successive applies
# (or re-runs after a partial failure) don't collide with orphan IdPs left
# on the backend. Stored in state, so `apply` is idempotent within one
# demo lifecycle; `destroy` clears it.
resource "random_id" "demo_suffix" {
  byte_length = 4 # 8 hex chars
}

provider "archestra" {
  # base_url + api_key are read from ARCHESTRA_BASE_URL / ARCHESTRA_API_KEY.
  # Don't commit keys to source.
}

resource "archestra_organization_settings" "main" {
  app_name    = var.app_name
  footer_text = var.footer_text

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





  default_llm_api_key_id = archestra_model_provider.ollama_vault.id
  default_model_id       = archestra_model.llama3.id # the platform pairs the default key with a model row
  default_agent_id       = archestra_agent.support.id

  # NOTE: embedding_* and reranker_* are deliberately omitted from this demo.
  # The platform treats them as write-once — changing the embedding model after
  # any KB has been indexed requires server-side cleanup. Set them in your own
  # module on initial bring-up only.
}
