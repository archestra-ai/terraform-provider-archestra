# Step 1: register the MCP server in the private catalog. The catalog item
# captures *how* to run the server; the install captures *that* it runs.
resource "archestra_mcp_registry_catalog_item" "filesystem" {
  name        = "filesystem"
  description = "Read-only filesystem MCP server"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

    environment = [
      { key = "NODE_ENV", type = "plain_text", value = "production" },
    ]
  }
}

# Step 2: install it. Tools are discovered asynchronously; once the install
# settles, `tool_id_by_name` is populated for one-line lookups elsewhere.
resource "archestra_mcp_server_installation" "filesystem" {
  name       = "filesystem"
  catalog_id = archestra_mcp_registry_catalog_item.filesystem.id
}

# Install with auth fields supplied — the catalog item declared `auth_fields`
# so the install must pass values via `access_token` (or environment values).
resource "archestra_mcp_registry_catalog_item" "github" {
  name             = "github"
  description      = "GitHub MCP server"
  auth_description = "Requires a GitHub PAT"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-github"]
  }

  auth_fields = [
    {
      name     = "GITHUB_TOKEN"
      label    = "GitHub PAT"
      type     = "password"
      required = true
    }
  ]
}

resource "archestra_mcp_server_installation" "github" {
  name         = "github"
  catalog_id   = archestra_mcp_registry_catalog_item.github.id
  access_token = var.github_pat # Secret — pass via TF_VAR_github_pat or a vault data source.
}

# Team-scoped install + per-install user_config_values for catalog items that
# expose `user_config`. Maps go through jsonencode so types round-trip.
resource "archestra_mcp_server_installation" "filesystem_team" {
  name       = "filesystem-eng"
  catalog_id = archestra_mcp_registry_catalog_item.with_user_config.id
  team_id    = archestra_team.engineering.id

  user_config_values = {
    workspace    = jsonencode("/home/eng")
    max_results  = jsonencode(100)
    enable_cache = jsonencode(false)
  }

  # Pre-bind the install to a list of agents so they can see its tools.
  agent_ids = [archestra_agent.support.id]
}

# BYOS install — point the backend at a Vault path instead of inlining the
# token. Requires `ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT` on the backend.
resource "archestra_mcp_server_installation" "github_vault" {
  name          = "github-vault"
  catalog_id    = archestra_mcp_registry_catalog_item.github.id
  is_byos_vault = true
  secret_id     = "secret/data/archestra/mcp/github" # Vault path, not raw token.
}
