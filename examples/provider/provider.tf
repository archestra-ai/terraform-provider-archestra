terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
      # `~> 0.6.0` reads as `>= 0.6.0, < 0.7.0` — pins to the 0.6.x patch line.
      # The provider is pre-1.0; minor bumps may include breaking changes, so
      # widen the constraint deliberately after reviewing the changelog.
      version = "~> 0.6.0"
    }
  }
}

provider "archestra" {
  # base_url and api_key are read from the environment by default —
  # don't commit secrets to source.
  #   export ARCHESTRA_BASE_URL="https://archestra.your-company.example"
  #   export ARCHESTRA_API_KEY="arch_..."   # mint via Settings → API Keys
}
