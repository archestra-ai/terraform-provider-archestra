# External connector that syncs documents into Archestra knowledge
# bases on a schedule. The `config` JSON object is connector-type-
# specific; the schema for each type lives in the platform repo at
# `platform/backend/src/types/knowledge-connector.ts`.
#
# The provider validates the required keys for the most common types
# at plan time (jira, confluence, github, gitlab, servicenow,
# sharepoint, asana). Other types (notion, gdrive, dropbox, linear)
# accept config with no required keys — the backend zod validator
# handles them.

# GitHub — most common shape. `api_token` is a personal access token
# with `repo` scope.
resource "archestra_knowledge_connector" "engineering_repos" {
  name           = "Engineering GitHub"
  connector_type = "github"

  config = jsonencode({
    githubUrl            = "https://github.com"
    owner                = "archestra-ai"
    repos                = ["platform", "terraform-provider-archestra"]
    includeIssues        = true
    includePullRequests  = true
    includeMarkdownFiles = true
  })

  credentials = {
    api_token = var.github_pat
  }

  schedule           = "0 6 * * *"
  knowledge_base_ids = [archestra_knowledge_base.engineering.id]
}

# Jira Cloud — `email` AND `api_token` are both required by Atlassian's
# token auth; provide both.
resource "archestra_knowledge_connector" "support_tickets" {
  name           = "Support Tickets"
  connector_type = "jira"

  config = jsonencode({
    jiraBaseUrl = "https://acme.atlassian.net"
    isCloud     = true
    projectKey  = "SUP"
  })

  credentials = {
    email     = "automation@acme.com"
    api_token = var.atlassian_token
  }

  knowledge_base_ids = [archestra_knowledge_base.support.id]
}

# Team-scoped Notion workspace. Notion has no required config keys —
# omit `databaseIds` and `pageIds` to ingest the whole accessible
# workspace.
resource "archestra_knowledge_connector" "team_notion" {
  name           = "Engineering Notion"
  connector_type = "notion"
  visibility     = "team-scoped"
  team_ids       = [archestra_team.engineering.id]

  config = jsonencode({})

  credentials = {
    api_token = var.notion_integration_token
  }
}
