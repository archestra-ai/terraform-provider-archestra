# Reference a registry entry added in the UI by name.
data "archestra_mcp_registry_entry" "github" {
  name = "GitHub"
}

output "github_registry_entry_id" {
  value = data.archestra_mcp_registry_entry.github.id
}
