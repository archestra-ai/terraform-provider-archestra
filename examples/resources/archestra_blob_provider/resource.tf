# An S3 bucket the platform reaches with its own AWS credential chain.
resource "archestra_blob_provider" "finops" {
  name = "FinOps exports"

  s3 = {
    bucket      = "acme-finops"
    region      = "us-east-1"
    key_prefix  = "archestra"
    auth_method = "default_chain"
  }
}

# Google Cloud Storage with a service account key. Credentials are write-only:
# the API never returns them, and removing them clears the stored key.
resource "archestra_blob_provider" "gcs" {
  name = "Billing bucket"

  gcs = {
    bucket      = "acme-billing"
    auth_method = "service_account_key"
  }

  credentials = {
    serviceAccountKey = var.gcs_service_account_key
  }
}

variable "gcs_service_account_key" {
  type      = string
  sensitive = true
}
