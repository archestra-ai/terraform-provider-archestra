# A knowledge connector syncs documents from an external source into
# knowledge bases. `config` takes the source's settings as JSON with the
# API's camelCase keys; leave out `type`, the provider adds it from
# `connector_type`.
#
# Destroying a connector moves it to the trash. Documents, sync runs and
# per-member overrides are managed in the UI.

resource "archestra_knowledge_base" "docs" {
  name = "Product Docs"
}

# Web crawler: needs no credentials.
resource "archestra_knowledge_connector" "docs_site" {
  name           = "Docs site"
  connector_type = "web_crawler"
  config = jsonencode({
    startUrl = "https://docs.example.com"
    maxPages = 200
  })
  schedule           = "0 3 * * *"
  knowledge_base_ids = [archestra_knowledge_base.docs.id]
}

# Jira: credentials are write-only and never read back.
variable "jira_api_token" {
  type      = string
  sensitive = true
}

resource "archestra_knowledge_connector" "jira" {
  name           = "Engineering Jira"
  connector_type = "jira"
  config = jsonencode({
    jiraBaseUrl = "https://example.atlassian.net"
    isCloud     = true
    projectKey  = "ENG"
  })
  credentials = {
    email     = "bot@example.com"
    api_token = var.jira_api_token
  }
  knowledge_base_ids = [archestra_knowledge_base.docs.id]
}

# Google Drive in individual mode: created without credentials, then
# authorized once in the UI (needs the deployment's Google OAuth client).
resource "archestra_knowledge_connector" "drive" {
  name           = "Team Drive"
  connector_type = "gdrive"
  config = jsonencode({
    authMode = "oauth"
  })
}
