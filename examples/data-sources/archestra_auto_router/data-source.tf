# ALPHA. Look up a router created in the UI by the model name clients request.
data "archestra_auto_router" "default" {
  name = "auto"
}

# Make it the organization default for agents without their own model.
resource "archestra_organization_settings" "this" {
  default_auto_router_id = data.archestra_auto_router.default.id
}

output "auto_router_tiers" {
  value = data.archestra_auto_router.default.tiers
}
