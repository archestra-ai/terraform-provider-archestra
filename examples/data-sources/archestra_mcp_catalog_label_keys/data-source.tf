# Distinct label keys in the MCP registry catalog. Useful for asserting
# a label-based catalog item reference matches at least one item.
data "archestra_mcp_catalog_label_keys" "all" {}

output "catalog_label_keys" {
  value = data.archestra_mcp_catalog_label_keys.all.keys
}
