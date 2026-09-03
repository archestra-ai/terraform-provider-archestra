# Look up a knowledge base by ID — useful when the KB was created
# out-of-band (e.g., through the platform UI) and you want to wire
# it into a Terraform-managed agent.

data "archestra_knowledge_base" "imported" {
  id = "11111111-1111-1111-1111-111111111111"
}

resource "archestra_agent" "support" {
  name               = "support"
  llm_model          = "llama3"
  llm_api_key_id     = archestra_llm_provider_api_key.ollama.id
  knowledge_base_ids = [data.archestra_knowledge_base.imported.id]
}
