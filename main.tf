terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
    }
  }
}

resource "archestra_sso_provider" "google_oidc" {
  provider_id = "google-sso"
  domain      = "example.com"

  oidc_config = {
    client_id          = "YOUR_CLIENT_ID"
    client_secret      = "YOUR_CLIENT_SECRET"
    discovery_endpoint = "https://accounts.google.com/.well-known/openid-configuration"
    issuer             = "https://accounts.google.com"
    scopes             = ["openid", "email", "profile"]
  }

  role_mapping = {
    default_role = "member"
    rules = [
      {
        role       = "admin"
        expression = "{{#contains email '@example.com'}}true{{/contains}}"
      }
    ]
  }
}