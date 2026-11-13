# Reference a blob provider created in the UI by name.
data "archestra_blob_provider" "finops" {
  name = "FinOps exports"
}

resource "archestra_cost_export" "hourly" {
  name             = "Hourly LLM spend"
  blob_provider_id = data.archestra_blob_provider.finops.id
  schedule         = "hourly"
}
