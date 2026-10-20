resource "archestra_app" "runbook" {
  name                = "Runbook"
  html                = file("${path.module}/app.html")
  browser_permissions = []
  enabled             = true
  locked              = false
  open_in_fullscreen  = false
  labels              = [{ key = "purpose", value = "operations" }]

  # Tool assignments, when needed, belong on the App:
  # tools = [{ tool_id = data.archestra_tool.lookup.id,
  #            credential_resolution_mode = "dynamic" }]
}
