# Local MCP server with stdio transport (default)
resource "archestra_mcp_registry_catalog_item" "filesystem" {
  name        = "filesystem-mcp-server"
  description = "MCP server for filesystem operations"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/home/user"]

    environment = {
      NODE_ENV = "production"
    }
  }
}

# Local MCP server with custom auth fields
resource "archestra_mcp_registry_catalog_item" "github" {
  name                 = "github-mcp-server"
  description          = "MCP server for GitHub API operations"
  docs_url             = "https://github.com/modelcontextprotocol/servers/tree/main/src/github"
  auth_description     = "Requires a GitHub personal access token"
  installation_command = "npm install -g @modelcontextprotocol/server-github"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-github"]

    environment = {
      NODE_ENV = "production"
    }
  }

  auth_fields = [
    {
      name        = "GITHUB_TOKEN"
      label       = "GitHub Personal Access Token"
      type        = "password"
      required    = true
      description = "Personal access token with repo and user scopes"
    }
  ]
}

# Local MCP server with streamable-http transport
resource "archestra_mcp_registry_catalog_item" "web_search" {
  name        = "web-search-mcp-server"
  description = "MCP server for web search using Brave Search API"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/brave-search"

  local_config = {
    command        = "node"
    arguments      = ["dist/index.js"]
    transport_type = "streamable-http"
    http_port      = 3000
    http_path      = "/sse"
  }

  auth_fields = [
    {
      name        = "BRAVE_API_KEY"
      label       = "Brave Search API Key"
      type        = "password"
      required    = true
      description = "API key from Brave Search API"
    }
  ]
}

# Local MCP server with Docker
resource "archestra_mcp_registry_catalog_item" "postgres" {
  name        = "postgres-mcp-server"
  description = "MCP server for PostgreSQL database operations"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/postgres"

  local_config = {
    command      = "npx"
    arguments    = ["-y", "@modelcontextprotocol/server-postgres"]
    docker_image = "postgres:16-alpine"

    environment = {
      POSTGRES_USER     = "admin"
      POSTGRES_PASSWORD = "${var.postgres_password}"
      POSTGRES_DB       = "myapp"
    }
  }

  auth_fields = [
    {
      name        = "DATABASE_URL"
      label       = "PostgreSQL Connection String"
      type        = "text"
      required    = true
      description = "Connection string for PostgreSQL database"
    }
  ]
}

# Remote MCP server with PAT authentication
resource "archestra_mcp_registry_catalog_item" "github_remote" {
  name        = "github-mcp-server-remote"
  description = "GitHub's official remote MCP Server"
  docs_url    = "https://github.com/github/github-mcp-server"

  remote_config = {
    url = "https://api.githubcopilot.com/mcp/"
  }

  auth_fields = [
    {
      name        = "GITHUB_PERSONAL_ACCESS_TOKEN"
      label       = "GitHub Personal Access Token"
      type        = "password"
      required    = true
      description = "GitHub PAT with appropriate repository permissions"
    }
  ]
}

# Local MCP server exposing installer-configurable user_config fields
resource "archestra_mcp_registry_catalog_item" "with_user_config" {
  name        = "user-config-mcp-server"
  description = "MCP server that collects configuration from the installer at install time"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }

  user_config = {
    workspace = {
      title       = "Workspace Path"
      description = "Absolute path to the workspace root"
      type        = "directory"
      required    = true
    }
    max_results = {
      title       = "Max Results"
      description = "Maximum number of records to return"
      type        = "number"
      default     = jsonencode(50)
      min         = 1
      max         = 500
    }
    enable_cache = {
      title       = "Enable Cache"
      description = "Whether to cache API responses"
      type        = "boolean"
      default     = jsonencode(true)
    }
  }
}

# Remote MCP server with OAuth authentication
resource "archestra_mcp_registry_catalog_item" "remote_oauth" {
  name        = "remote-oauth-mcp-server"
  description = "Remote MCP Server with OAuth authentication"
  docs_url    = "https://example.com/mcp-server"

  remote_config = {
    url = "https://api.example.com/mcp/"
    oauth_config = {
      client_id                  = "your-client-id"
      redirect_uris              = ["https://frontend.archestra.dev/oauth-callback"]
      scopes                     = ["read", "write"]
      supports_resource_metadata = true
    }
  }
}

# Remote MCP server with an advanced OAuth configuration — exercises all of the
# new fields exposed in v1.2.20: grant_type, audience, endpoint overrides,
# well-known discovery URL, default scopes, and provider metadata.
resource "archestra_mcp_registry_catalog_item" "remote_oauth_advanced" {
  name        = "remote-oauth-advanced"
  description = "Remote MCP server with an explicit OAuth authorization_code flow"

  remote_config = {
    url = "https://api.example.com/mcp/"
    oauth_config = {
      client_id              = "your-client-id"
      client_secret          = var.oauth_client_secret
      grant_type             = "authorization_code"
      redirect_uris          = ["https://frontend.archestra.dev/oauth-callback"]
      scopes                 = ["read", "write"]
      default_scopes         = ["openid"]
      audience               = "https://api.example.com"
      authorization_endpoint = "https://auth.example.com/oauth/authorize"
      token_endpoint         = "https://auth.example.com/oauth/token"
      well_known_url         = "https://auth.example.com/.well-known/oauth-authorization-server"
      provider_name          = "Example OAuth"
      browser_auth           = true
      generic_oauth          = false
    }
  }
}

# Local MCP server pulling from a private registry using inline credentials
# (alternative to referencing a pre-existing Kubernetes secret by name).
resource "archestra_mcp_registry_catalog_item" "private_registry" {
  name        = "private-registry-mcp-server"
  description = "MCP server whose image is pulled from a private registry using inline credentials"

  local_config = {
    command      = "/usr/local/bin/mcp-server"
    docker_image = "registry.example.com/team/mcp-server:1.0.0"
    image_pull_secrets = [
      {
        source   = "credentials"
        server   = "registry.example.com"
        username = var.registry_username
        password = var.registry_password
        email    = "devops@example.com"
      }
    ]
  }
}
