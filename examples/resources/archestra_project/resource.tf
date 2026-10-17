resource "archestra_project" "reports" {
  name        = "Reports"
  description = "Scheduled reporting work"
  labels      = [{ key = "purpose", value = "reporting" }]
}
