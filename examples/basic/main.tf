terraform {
  required_providers {
    archestra = {
      source  = "archestra-ai/archestra"
      version = "~> 3.0"
    }
  }
}

provider "archestra" {
  # base_url + api_key are read from ARCHESTRA_BASE_URL / ARCHESTRA_API_KEY.
  # Don't commit keys to source.
}

resource "archestra_team" "main" {
  name        = "basic-demo"
  description = "Team created by the basic getting-started demo."
}

resource "archestra_model_provider" "ollama" {
  name              = "Basic-Demo Ollama"
  llm_provider      = "ollama"
  vault_secret_path = "secret/data/test/ollama"
  vault_secret_key  = "api_key"
}

resource "archestra_mcp_registry_entry" "memory" {
  name        = "basic-demo-memory"
  description = "In-memory key-value store for the getting-started demo."
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/memory"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-memory"]
  }
}

resource "archestra_mcp_server" "memory" {
  name       = "basic-demo-memory"
  catalog_id = archestra_mcp_registry_entry.memory.id
}

# The platform pairs an agent's model with its provider key, so the agent
# references a model row and the key that serves it.
resource "archestra_model" "llama3" {
  model_id = "llama3"
}

resource "archestra_agent" "main" {
  tools          = [{ tool_id = archestra_mcp_server.memory.tool_id_by_name["${archestra_mcp_registry_entry.memory.name}__create_entities"], mcp_server_id = archestra_mcp_server.memory.id }]
  name           = "basic-demo-agent"
  description    = "Hello-world agent for the getting-started demo."
  system_prompt  = "You are a friendly assistant. Be concise."
  model_id       = archestra_model.llama3.id
  llm_api_key_id = archestra_model_provider.ollama.id
}


output "agent_id" {
  value = archestra_agent.main.id
}

output "memory_install_id" {
  value = archestra_mcp_server.memory.id
}
