resource "archestra_knowledge_directory" "handbook" {
  name = "Handbook"
}

resource "archestra_knowledge_file" "runbook" {
  filename       = "runbook.md"
  mime_type      = "text/markdown"
  content_base64 = base64encode("# Runbook\nReview the deployment before applying changes.\n")
  directory_id   = archestra_knowledge_directory.handbook.id
  labels         = [{ key = "purpose", value = "operations" }]
  permissions    = [{ subject_type = "organization", subject_id = "*", actions = ["read"] }]
}
