# Reference a knowledge base created in the UI by name.
data "archestra_knowledge_base" "handbook" {
  name = "Employee handbook"
}

output "handbook_knowledge_base_id" {
  value = data.archestra_knowledge_base.handbook.id
}
