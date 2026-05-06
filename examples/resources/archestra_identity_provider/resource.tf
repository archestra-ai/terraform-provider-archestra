# --- Placeholders to replace before applying ---
# All `*.example.com` URLs, `client_id`/`client_secret`, the SAML `cert`,
# and the inline IdP/SP metadata XML below are illustrative shapes only.
# Replace each with values from your real IdP before `terraform apply`.
# Pass `client_secret` and any private keys via TF variables, not literals.

# OIDC provider example (Keycloak / Generic OAuth)
resource "archestra_identity_provider" "oidc" {
  provider_id = "Keycloak"
  domain      = "example.com"                              # Replace with your verified domain
  issuer      = "https://keycloak.example.com/realms/main" # Replace with your IdP issuer

  oidc_config {
    issuer             = "https://keycloak.example.com/realms/main"
    discovery_endpoint = "https://keycloak.example.com/realms/main/.well-known/openid-configuration"
    client_id          = "archestra"
    client_secret      = var.oidc_client_secret # Declare: variable "oidc_client_secret" { sensitive = true }
    scopes             = ["openid", "email", "profile"]
    pkce               = false

    mapping {
      name  = "name"
      email = "email"
      image = "picture"
    }
  }

  role_mapping {
    default_role = "member"
    rules = [{
      expression = "{{#includes groups \"archestra-admins\"}}true{{/includes}}"
      role       = "admin"
    }]
  }

  team_sync_config {
    enabled           = true
    groups_expression = "{{#each groups}}{{this}},{{/each}}"
  }
}

# SAML provider example (Okta SAML)
resource "archestra_identity_provider" "saml" {
  provider_id = "OktaSAML"
  domain      = "example.com"
  issuer      = "https://example.com"

  saml_config {
    issuer       = "https://example.com"
    entry_point  = "https://okta.example.com/app/sso/saml"
    callback_url = "https://archestra.example.com/api/auth/sso/saml2/sp/acs/OktaSAML"
    # PLACEHOLDER — replace with your IdP's signing certificate (PEM).
    # Pass via a variable: variable "saml_cert" { type = string }
    cert              = var.saml_cert
    audience          = "https://archestra.example.com"
    digest_algorithm  = "sha256"
    identifier_format = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"

    # Extra parameters forwarded to the IdP alongside the SAML AuthnRequest.
    # Use jsonencode so booleans and numbers round-trip losslessly.
    additional_params = jsonencode({
      ForceAuthn = true
      MaxAge     = 3600
    })

    idp_metadata {
      entity_id = "https://okta.example.com"
      # IdPs typically distribute metadata as an .xml file. Inlining the
      # string keeps this example self-contained, but in real modules:
      #   metadata = file("${path.module}/idp-metadata.xml")
      metadata = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" entityID=\"https://okta.example.com\"><IDPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"><SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect\" Location=\"https://okta.example.com/app/sso/saml\"/></IDPSSODescriptor></EntityDescriptor>"

      single_sign_on_service {
        binding  = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
        location = "https://okta.example.com/app/sso/saml"
      }
    }

    sp_metadata {
      entity_id = "https://archestra.example.com"
      # Or: metadata = file("${path.module}/sp-metadata.xml")
      metadata = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" entityID=\"https://archestra.example.com\"><SPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"><AssertionConsumerService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://archestra.example.com/api/auth/sso/saml2/sp/acs/OktaSAML\" index=\"0\" isDefault=\"true\"/></SPSSODescriptor></EntityDescriptor>"
    }

    mapping {
      email      = "email"
      first_name = "firstName"
      last_name  = "lastName"
      name       = "name"
      id         = "employeeId"
    }
  }

  role_mapping {
    default_role = "member"
  }
}
