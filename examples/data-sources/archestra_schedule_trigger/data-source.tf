# Lookup a known schedule trigger by ID — useful for audit scripts
# inspecting cron / last-executed metadata without managing the trigger.
data "archestra_schedule_trigger" "nightly" {
  id = "22222222-2222-4222-a222-222222222222"
}

output "nightly_cron" {
  value = data.archestra_schedule_trigger.nightly.cron_expression
}

output "nightly_last_run" {
  value = data.archestra_schedule_trigger.nightly.last_executed_at
}
