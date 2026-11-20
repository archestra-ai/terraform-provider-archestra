data "archestra_identity_provider" "okta" {
  provider_id = "okta"
}

output "okta_issuer" {
  value = data.archestra_identity_provider.okta.issuer
}
