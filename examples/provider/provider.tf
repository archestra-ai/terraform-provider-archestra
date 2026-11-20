terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
      # Read MIGRATION-v3.md before upgrading from 2.x.
      version = "~> 3.0"
    }
  }
}

provider "archestra" {
  # The Archestra release you approved. See the API compatibility guide.
  api_version_constraint = "= 1.4.1"

  # base_url and api_key are read from the environment by default —
  # don't commit secrets to source.
  #   export ARCHESTRA_BASE_URL="https://archestra.your-company.example"
  #   export ARCHESTRA_API_KEY="arch_..."   # mint via Settings → API Keys
  #
  # Or sign in with workload identity instead of an API key:
  #   export ARCHESTRA_SERVICE_ACCOUNT_ID="<service account ID>"
  #   export ARCHESTRA_OIDC_TOKEN_FILE="/var/run/secrets/archestra/serviceaccount/token"
}
