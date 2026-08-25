# --- Knowledge bases -------------------------------------------------------
#
# A knowledge base groups documents and connectors that agents can search
# over. Attach to an agent via its `knowledge_base_ids` attribute (see
# agents.tf for the wire).
#
# Connectors that sync external systems (Jira, GitHub, Notion, etc.) into
# these knowledge bases are separate `archestra_knowledge_connector`
# resources — shipping on their own branch.

resource "archestra_knowledge_base" "support_docs" {
  name        = "Support Docs"
  description = "Public KB articles, runbooks, and SOPs used by the customer-support agent."
}

resource "archestra_knowledge_base" "internal_wiki" {
  name = "Internal Wiki"
}
