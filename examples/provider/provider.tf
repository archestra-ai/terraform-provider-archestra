terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
      # Proposed v3 release; review MIGRATION-v3.md before upgrading.
      # v3 is not published yet. Use a local development override to preview it.
      version = "~> 3.0.0"
    }
  }
}

provider "archestra" {
  api_version_constraint = "= 1.4.0-rc.49"

  # base_url and api_key are read from the environment by default —
  # don't commit secrets to source.
  #   export ARCHESTRA_BASE_URL="https://archestra.your-company.example"
  #   export ARCHESTRA_API_KEY="arch_..."   # mint via Settings → API Keys
}
