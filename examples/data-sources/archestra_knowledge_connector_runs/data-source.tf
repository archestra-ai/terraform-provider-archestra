# Read the sync-run history for a connector. Returns up to
# `max_records` (default 1000) most-recent runs.

data "archestra_knowledge_connector_runs" "recent" {
  connector_id = archestra_knowledge_connector.jira.id
  max_records  = 25
}

# Surface only the failed runs for alerting.
output "recent_failed_runs" {
  value = [
    for r in data.archestra_knowledge_connector_runs.recent.runs :
    r if r.status == "failed"
  ]
}
