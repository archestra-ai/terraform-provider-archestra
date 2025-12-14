# Basic SSO Provider Configuration
resource "archestra_sso_provider" "google" {
  provider_id = "google"
  issuer      = "https://accounts.google.com"
  domain      = "example.com"
}

# OIDC SSO Provider with full configuration
resource "archestra_sso_provider" "okta" {
  provider_id = "okta"
  issuer      = "https://your-org.okta.com"
  domain      = "example.com"
}

# SAML SSO Provider
resource "archestra_sso_provider" "saml" {
  provider_id = "saml"
  issuer      = "https://your-idp.com"
  domain      = "example.com"
}

# Data source example
data "archestra_sso_provider" "existing" {
  id = "sso-provider-id"
}