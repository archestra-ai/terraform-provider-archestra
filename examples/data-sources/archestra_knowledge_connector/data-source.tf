# Look up an existing knowledge connector. The polymorphic `config`
# is returned as a JSON string with the discriminator `type` stripped
# — round-trips cleanly into a resource block if you decide to import.

data "archestra_knowledge_connector" "jira" {
  id = "22222222-2222-2222-2222-222222222222"
}

output "jira_config" {
  value     = data.archestra_knowledge_connector.jira.config
  sensitive = false
}
