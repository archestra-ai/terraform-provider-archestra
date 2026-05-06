# --- Agents ----------------------------------------------------------------

resource "archestra_agent" "support" {
  name           = "customer-support"
  description    = "Tier-1 customer support agent"
  system_prompt  = "You are a friendly customer-support agent. Be concise."
  llm_model      = "llama3"
  llm_api_key_id = archestra_llm_provider_api_key.ollama_vault.id
  scope          = "org"
}

resource "archestra_agent" "support_batch" {
  name           = "customer-support-batch"
  description    = "Variant agent for bulk tool-assignment test"
  system_prompt  = "Same as support agent — separate so we can exercise agent_tool_batch without colliding with read_text_file's per-tool wiring."
  llm_model      = "llama3"
  llm_api_key_id = archestra_llm_provider_api_key.ollama_vault.id
  scope          = "org"
}

resource "archestra_agent" "intake" {
  name           = "support-intake"
  description    = "Email-triggered intake agent — routes incoming customer emails into chats"
  system_prompt  = "Triage incoming customer emails into the right team."
  llm_model      = "llama3"
  llm_api_key_id = archestra_llm_provider_api_key.ollama_vault.id
  scope          = "org"

  consider_context_untrusted    = true
  incoming_email_enabled        = true
  incoming_email_security_mode  = "internal"
  incoming_email_allowed_domain = "demo.example.com"

  suggested_prompts = [
    {
      summary_title = "Refund a charge"
      prompt        = "Help me refund a customer charge."
    },
    {
      summary_title = "Look up an order"
      prompt        = "Find the most recent order for customer {{email}}."
    },
  ]

  labels = [
    { key = "team", value = "support" },
    { key = "tier", value = "1" },
    { key = "channel", value = "email" },
  ]
}

# --- Tool wirings ----------------------------------------------------------

resource "archestra_agent_tool" "read_text_file" {
  agent_id      = archestra_agent.support.id
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
  tool_id       = archestra_mcp_server_installation.filesystem.tool_id_by_name["${archestra_mcp_registry_catalog_item.filesystem.name}__read_text_file"]
}

resource "archestra_agent_tool_batch" "support_batch_filesystem" {
  agent_id      = archestra_agent.support_batch.id
  mcp_server_id = archestra_mcp_server_installation.filesystem.id
  tool_ids      = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])
}
