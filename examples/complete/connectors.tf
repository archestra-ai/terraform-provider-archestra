# --- Knowledge connectors --------------------------------------------------
#
# Connectors sync external documents (GitHub repos, Notion workspaces,
# Atlassian tickets, etc.) into one or more `archestra_knowledge_base`
# records on a schedule. The full per-type config schema lives in the
# platform repo at `platform/backend/src/types/knowledge-connector.ts`.
#
# `knowledge_base_ids` reconciliation is handled in-place by the
# resource via the backend's assign/unassign endpoints; the KB resource
# itself lands on its own branch (`feat/archestra-knowledge-base`),
# so this demo only declares connectors. Once both branches merge to
# main, wire `knowledge_base_ids = [archestra_knowledge_base.X.id, ...]`.
#
# `enabled = false` on every connector below — the demo can be applied
# safely without seeded external credentials. Flip to `true` after
# verifying your real API tokens.

# Notion is the minimum-friction shape: no required config keys, just
# an integration token from your Notion workspace settings.
resource "archestra_knowledge_connector" "notion_demo" {
  name           = "Demo Notion Workspace"
  description    = "Pulls the demo workspace's pages into Archestra for the support agent to ground its answers."
  connector_type = "notion"

  config = jsonencode({})

  credentials = {
    api_token = var.notion_integration_token
  }

  enabled = false
}

# Jira Cloud uses email + token auth and requires `jiraBaseUrl` +
# `isCloud` in config. The provider's plan-time validator catches
# missing required keys per type.
resource "archestra_knowledge_connector" "jira_demo" {
  name           = "Demo Atlassian Cloud"
  connector_type = "jira"

  config = jsonencode({
    jiraBaseUrl = "https://demo.atlassian.net"
    isCloud     = true
    projectKey  = "SUP"
  })

  credentials = {
    email     = "automation@demo.example.com"
    api_token = var.atlassian_api_token
  }

  enabled = false
}
