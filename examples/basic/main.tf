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

resource "archestra_team" "main" {
  name        = "basic-demo"
  description = "Team created by the basic getting-started demo."
}

resource "archestra_llm_provider_api_key" "ollama" {
  name              = "Basic-Demo Ollama"
  llm_provider      = "ollama"
  vault_secret_path = "secret/data/test/ollama"
  vault_secret_key  = "api_key"
  scope             = "org"
}

resource "archestra_mcp_registry_catalog_item" "memory" {
  name        = "basic-demo-memory"
  description = "In-memory key-value store for the getting-started demo."
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/memory"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-memory"]
  }
}

resource "archestra_mcp_server_installation" "memory" {
  name       = "basic-demo-memory"
  catalog_id = archestra_mcp_registry_catalog_item.memory.id
}

resource "archestra_agent" "main" {
  name           = "basic-demo-agent"
  description    = "Hello-world agent for the getting-started demo."
  system_prompt  = "You are a friendly assistant. Be concise."
  llm_model      = "llama3"
  llm_api_key_id = archestra_llm_provider_api_key.ollama.id
  scope          = "org"
}

resource "archestra_agent_tool" "create_entities" {
  agent_id      = archestra_agent.main.id
  mcp_server_id = archestra_mcp_server_installation.memory.id
  tool_id       = archestra_mcp_server_installation.memory.tool_id_by_name["${archestra_mcp_registry_catalog_item.memory.name}__create_entities"]
}

output "agent_id" {
  value = archestra_agent.main.id
}

output "memory_install_id" {
  value = archestra_mcp_server_installation.memory.id
}
