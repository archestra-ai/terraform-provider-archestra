resource "archestra_organization_settings" "main" {
  font                         = "roboto"
  color_theme                  = "ocean-breeze"
  limit_cleanup_interval       = "1w"
  compression_scope            = "organization"
  onboarding_complete          = true
  convert_tool_results_to_toon = false

  # Logo is sensitive and should typically be passed via variable
  # logo = var.organization_logo
}
