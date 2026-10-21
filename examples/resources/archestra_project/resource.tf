resource "archestra_project" "reports" {
  name        = "Reports"
  description = "Scheduled reporting work"
  labels      = [{ key = "purpose", value = "reporting" }]
  # Authoritative app links; omit to leave links unmanaged, [] unlinks all.
  app_ids = [archestra_app.runbook.id]
}
