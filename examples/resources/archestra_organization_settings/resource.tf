resource "archestra_organization_settings" "example" {
  font                         = "inter"
  color_theme                  = "modern-minimal"
  compression_scope            = "organization"
  onboarding_complete          = true
  convert_tool_results_to_toon = true
  limit_cleanup_interval       = "24h"
}
