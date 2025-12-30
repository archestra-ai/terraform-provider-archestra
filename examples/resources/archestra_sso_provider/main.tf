provider "archestra" {
  api_key  = "test-api-key"
  base_url = "http://localhost:9000"
}

resource "archestra_sso_provider" "example" {
  provider_id = "google"
  domain      = "example.com"
  issuer      = "https://accounts.google.com"

  oidc_config {
    client_id              = "example-client-id"
    client_secret          = "example-secret"
    discovery_endpoint     = "https://accounts.google.com/.well-known/openid-configuration"
    authorization_endpoint = "https://accounts.google.com/o/oauth2/v2/auth"
    user_info_endpoint     = "https://openidconnect.googleapis.com/v1/userinfo"
    scopes                 = ["openid", "email", "profile"]
    pkce                   = true
  }
}
