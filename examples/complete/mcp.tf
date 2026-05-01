# --- Catalog items ---------------------------------------------------------

resource "archestra_mcp_registry_catalog_item" "filesystem" {
  name        = "filesystem"
  description = "Read-only filesystem MCP server"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

    environment = [
      { key = "NODE_ENV", type = "plain_text", value = "production" },
      { key = "MCP_LOG_LEVEL", type = "plain_text", value = "info" },
    ]
  }
}

resource "archestra_mcp_registry_catalog_item" "memory" {
  name        = "memory"
  description = "In-memory key-value store MCP server (no auth, safe for local testing)"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/memory"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-memory"]
  }
}

resource "archestra_mcp_registry_catalog_item" "everything" {
  name        = "mcp-everything"
  description = "Reference test MCP server — exposes every MCP feature, no auth required"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/everything"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-everything"]
  }
}

resource "archestra_mcp_registry_catalog_item" "configurable_fs" {
  name        = "configurable-fs"
  description = "Filesystem MCP with installer-supplied workspace + tuning"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem"]
  }

  user_config = {
    workspace = {
      title       = "Workspace path"
      description = "Absolute path the server is allowed to read."
      type        = "string"
      required    = true
    }
    max_results = {
      title       = "Max results"
      description = "Cap on results returned per call."
      type        = "number"
      default     = jsonencode(50)
    }
  }
}

# Remote MCP — exercises the remote_config (HTTP/SSE) transport instead of
# local stdio. gitmcp.io is a free, no-auth wrapper around any GitHub repo.
resource "archestra_mcp_registry_catalog_item" "gitmcp_servers" {
  name        = "gitmcp-mcp-servers"
  description = "gitmcp.io MCP wrapper around the modelcontextprotocol/servers repo — exercises the remote_config transport (HTTP/SSE) instead of local stdio. Free, no auth required."
  docs_url    = "https://gitmcp.io/"

  remote_config = {
    url = "https://gitmcp.io/modelcontextprotocol/servers"
  }
}

# --- Installations ---------------------------------------------------------

resource "archestra_mcp_server_installation" "filesystem" {
  name       = "filesystem"
  catalog_id = archestra_mcp_registry_catalog_item.filesystem.id
}

resource "archestra_mcp_server_installation" "memory" {
  name       = "memory"
  catalog_id = archestra_mcp_registry_catalog_item.memory.id
}

resource "archestra_mcp_server_installation" "memory_support" {
  name       = "memory-support"
  catalog_id = archestra_mcp_registry_catalog_item.memory.id
  team_id    = archestra_team.support.id
  agent_ids  = [archestra_agent.support.id]
}

resource "archestra_mcp_server_installation" "everything" {
  name       = "mcp-everything"
  catalog_id = archestra_mcp_registry_catalog_item.everything.id
}

resource "archestra_mcp_server_installation" "configurable_fs_engineering" {
  name       = "configurable-fs-engineering"
  catalog_id = archestra_mcp_registry_catalog_item.configurable_fs.id
  team_id    = archestra_team.engineering.id

  user_config_values = {
    workspace   = jsonencode("/tmp/eng-workspace")
    max_results = jsonencode(100)
  }
}

# --- Gateways --------------------------------------------------------------

resource "archestra_mcp_gateway" "engineering" {
  name        = "engineering-mcp"
  description = "MCP gateway scoped to engineering"
  scope       = "team"
  teams       = [archestra_team.engineering.id]
}

# JWT-authenticated MCP gateway — every inbound request must carry a token
# signed by the OIDC IdP. Exercises identity_provider_id wiring and
# passthrough_headers / labels enrichment.
resource "archestra_mcp_gateway" "secure" {
  name                 = "secure-mcp"
  description          = "Production gateway behind JWT auth"
  identity_provider_id = archestra_identity_provider.oidc.id

  passthrough_headers = ["x-correlation-id", "x-tenant-id", "x-request-id"]

  labels = [
    { key = "environment", value = "production" },
    { key = "auth", value = "jwt" },
    { key = "tier", value = "1" },
  ]
}
