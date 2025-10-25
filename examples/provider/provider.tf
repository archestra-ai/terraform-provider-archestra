terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
    }
  }
}

provider "archestra" {
  base_url = "http://localhost:9000" # Optional, defaults to http://localhost:9000
  api_key  = "your-api-key-here"     # Required - can also use ARCHESTRA_API_KEY env var
}
