# List every knowledge base in the organization (paginated
# exhaustively). Filter with `search` (case-insensitive substring on
# name/description).

data "archestra_knowledge_bases" "support_kbs" {
  search = "support"
}

output "support_kb_ids" {
  value = [for kb in data.archestra_knowledge_bases.support_kbs.knowledge_bases : kb.id]
}
