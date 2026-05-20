# Read execution history for a scheduled trigger.

data "archestra_schedule_trigger_runs" "daily_summary" {
  trigger_id  = archestra_schedule_trigger.daily_summary.id
  status      = "failed"
  max_records = 50
}

output "failed_daily_summary_count" {
  value = length(data.archestra_schedule_trigger_runs.daily_summary.runs)
}
