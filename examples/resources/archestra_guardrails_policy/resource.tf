# Keep the organization's guardrails policy in Git. Start from the current
# text under Settings → Guardrails, save it next to this file, and edit it
# there. This resource owns the whole text: manage battery installs in the
# file's `include` list rather than with archestra_openappa_battery_install.
resource "archestra_guardrails_policy" "this" {
  content = file("${path.module}/guardrails-policy.toml")
}
