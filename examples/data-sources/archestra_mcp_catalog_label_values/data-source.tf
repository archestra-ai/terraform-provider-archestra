# Values for a specific catalog-item label key. Drives dynamic catalog
# selection in HCL — e.g. picking every `category = "git"` catalog item
# without hard-coding their IDs.
data "archestra_mcp_catalog_label_values" "categories" {
  key = "category"
}

output "catalog_categories" {
  value = data.archestra_mcp_catalog_label_values.categories.values
}
