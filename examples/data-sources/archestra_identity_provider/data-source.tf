# Lookup an existing IdP by ID. Surfaces non-credential metadata
# (domain, issuer, has_oidc_config / has_saml_config flags); the GET
# endpoint returns clientSecret in plaintext but this data source
# deliberately omits secret fields — manage credentials via
# archestra_identity_provider.
data "archestra_identity_provider" "primary" {
  id = "okta-corp"
}

output "primary_idp_domain" {
  value = data.archestra_identity_provider.primary.domain
}

output "primary_idp_is_oidc" {
  value = data.archestra_identity_provider.primary.has_oidc_config
}
