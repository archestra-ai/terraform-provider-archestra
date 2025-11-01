# Local MCP server with stdio transport (default)
resource "archestra_mcp_server" "filesystem" {
  name        = "filesystem-mcp-server"
  description = "MCP server for filesystem operations"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem"

  local_config {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/home/user"]

    environment = {
      NODE_ENV = "production"
    }
  }
}

# Local MCP server with custom auth fields
resource "archestra_mcp_server" "github" {
  name                 = "github-mcp-server"
  description          = "MCP server for GitHub API operations"
  docs_url             = "https://github.com/modelcontextprotocol/servers/tree/main/src/github"
  auth_description     = "Requires a GitHub personal access token"
  installation_command = "npm install -g @modelcontextprotocol/server-github"

  local_config {
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

# Local MCP server with SSE transport
resource "archestra_mcp_server" "web_search" {
  name        = "web-search-mcp-server"
  description = "MCP server for web search using Brave Search API"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/brave-search"

  local_config {
    command        = "node"
    arguments      = ["dist/index.js"]
    transport_type = "sse"
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
resource "archestra_mcp_server" "postgres" {
  name        = "postgres-mcp-server"
  description = "MCP server for PostgreSQL database operations"
  docs_url    = "https://github.com/modelcontextprotocol/servers/tree/main/src/postgres"

  local_config {
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
