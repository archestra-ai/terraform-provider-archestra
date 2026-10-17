# A knowledge base groups documents that agents can search over.
# Attach to an `archestra_agent` via its `knowledge_base_ids`
# attribute.
#
# Prerequisites: a KB only returns results once
# `archestra_organization_settings` has `embedding_chat_api_key_id` +
# `embedding_model` and `reranker_chat_api_key_id` + `reranker_model`
# configured. Creation succeeds without them (mirrors the UI), but
# downstream searches stay empty.
#
# To clear an existing description on a subsequent apply, omit the
# attribute — the merge-patch then sends an explicit null.

resource "archestra_knowledge_base" "support_docs" {
  name        = "Support Docs"
  description = "Public KB articles, runbooks, and SOPs used by the customer support agent."
}

# Minimal — only `name` is required.
resource "archestra_knowledge_base" "internal_wiki" {
  name = "Internal Wiki"
}
