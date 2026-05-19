# Singleton: at most one incoming-email subscription per organization.
# The backend wipes any existing subscription on every setup call, so
# changing webhook_url is an in-place Update (no replacement).
#
# Provider must be configured at the org level first — without it the
# backend rejects the setup call with 400.

resource "archestra_incoming_email" "outlook" {
  webhook_url = "https://ingest.example.com/archestra/webhooks/email"
}

output "incoming_email_expires_at" {
  value = archestra_incoming_email.outlook.expires_at
}

output "incoming_email_provider" {
  value = archestra_incoming_email.outlook.email_provider
}
