# --- Agents ----------------------------------------------------------------

resource "archestra_agent" "support" {
  tools          = [{ tool_id = archestra_mcp_server.filesystem.tool_id_by_name["${archestra_mcp_registry_entry.filesystem.name}__read_text_file"], mcp_server_id = archestra_mcp_server.filesystem.id }]
  name           = "customer-support"
  description    = "Tier-1 customer support agent"
  system_prompt  = "You are a friendly customer-support agent. Be concise."
  llm_model      = "llama3"
  llm_api_key_id = archestra_model_provider.ollama_vault.id
}

resource "archestra_agent" "support_batch" {
  tools          = [for tool in archestra_mcp_server.filesystem.tools : { tool_id = tool.id, mcp_server_id = archestra_mcp_server.filesystem.id }]
  name           = "customer-support-batch"
  description    = "Agent with every tool from a selected MCP server"
  system_prompt  = "Answer support questions using the filesystem tools."
  llm_model      = "llama3"
  llm_api_key_id = archestra_model_provider.ollama_vault.id
}

resource "archestra_agent" "intake" {
  name           = "support-intake"
  description    = "Email-triggered intake agent — routes incoming customer emails into chats"
  system_prompt  = "Triage incoming customer emails into the right team."
  llm_model      = "llama3"
  llm_api_key_id = archestra_model_provider.ollama_vault.id

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
