resource "archestra_blob_provider" "finops" {
  name = "FinOps exports"

  s3 = {
    bucket      = "acme-finops"
    region      = "us-east-1"
    auth_method = "default_chain"
  }
}

# Daily FOCUS CSV files of LLM spend, backfilled from the start of the year.
resource "archestra_cost_export" "daily" {
  name             = "Daily LLM spend"
  blob_provider_id = archestra_blob_provider.finops.id
  schedule         = "daily"
  path_prefix      = "llm-costs"
  start_from       = "2026-01-01T00:00:00Z"
}
