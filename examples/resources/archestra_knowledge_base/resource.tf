# A knowledge base groups documents and connectors that agents can
# search over. Attach to an `archestra_agent` via its
# `knowledge_base_ids` attribute, and back it with one or more
# `archestra_knowledge_connector` resources (separate resource, follow
# up) for incremental sync.

resource "archestra_knowledge_base" "support_docs" {
  name        = "Support Docs"
  description = "Public KB articles, runbooks, and SOPs used by the customer support agent."
}

# Minimal — only `name` is required.
resource "archestra_knowledge_base" "internal_wiki" {
  name = "Internal Wiki"
}

# Wire a KB onto an agent's search surface.
#
# resource "archestra_agent" "support" {
#   name                = "support"
#   agent_type          = "agent"
#   knowledge_base_ids  = [archestra_knowledge_base.support_docs.id]
#   # ...
# }
