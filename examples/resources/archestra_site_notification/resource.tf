resource "archestra_site_notification" "maintenance" {
  content     = "Archestra is upgraded on Saturday from 09:00 to 10:00 UTC."
  expires_at  = "2030-01-04T10:00:00Z"
  dismissible = true
}
