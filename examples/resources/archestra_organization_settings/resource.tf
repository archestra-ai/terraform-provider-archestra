resource "archestra_organization_settings" "default" {
  font                         = "lato"
  color_theme                  = "amber-minimal"
  limit_cleanup_interval       = "12h"
  compression_scope            = "organization"
  onboarding_complete          = true
  convert_tool_results_to_toon = true

  # Optional LLM configurations (if supported by backend)
  # default_llm_config_id      = "..."
  # default_dual_llm_config_id = "..."
}
