# OIDC provider example (Keycloak / Generic OAuth)
resource "archestra_sso_provider" "oidc" {
  provider_id = "Keycloak"
  domain      = "example.com"
  issuer      = "https://keycloak.example.com/realms/main"

  oidc_config {
    issuer             = "https://keycloak.example.com/realms/main"
    discovery_endpoint = "https://keycloak.example.com/realms/main/.well-known/openid-configuration"
    client_id          = "archestra"
    client_secret      = "changeme"
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
}

# SAML provider example (Okta SAML)
resource "archestra_sso_provider" "saml" {
  provider_id = "OktaSAML"
  domain      = "example.com"
  issuer      = "https://example.com"

  saml_config {
    issuer            = "https://example.com"
    entry_point       = "https://okta.example.com/app/sso/saml"
    callback_url      = "https://archestra.example.com/api/auth/sso/saml2/sp/acs/OktaSAML"
    cert              = "-----BEGIN CERTIFICATE-----\nMIICiTCCAg+gAwIBAgIJAJ8l4HnPq7F8MAOGA1UEBhMCVVMxCzAJBgNVBAgTAkNB\nMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRowGAYDVQQKExFPcGVuQU1QIFNhbXBs\nZSBDb21wYW55IENBMRYwFAYDVQQDEx1vcGVuYW1wLmV4YW1wbGUuY29tMB4XDTE0\nMDgxOTE2MjQyNVoXDTIyMDgxODE2MjQyNVowdTELMAkGA1UEBhMCVVMxCzAJBgNV\nBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRowGAYDVQQKExFPcGVuQU1Q\nIFNhbXBsZSBDb21wYW55IENBMRYwFAYDVQQDEx1vcGVuYW1wLmV4YW1wbGUuY29t\nMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANgOqCbLsKv5CF+vGmJ9Vq5PJKKuiU8+\nLpqtHKHC9q3mRWxHF8dlE8j9D6Kz+N+CK+qGzFjWNBT3UVFzU5GJUYCAwEAAaNQ\nME4wHQYDVR0OBBYEFG7CJM9GjHn7Lqt8kJc8W5proUwWMB8GA1UdIwQYMBaAFG7C\nJM9GjHn7Lqt8kJc8W5proUwWMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQAD\nggEBABYIUUUeWDJ+wZF0lZ+mJnRnGZpXL2fKe3+KGjNM8xJfPf2YvqU4mgdMxgJn\n-----END CERTIFICATE-----"
    audience          = "https://archestra.example.com"
    digest_algorithm  = "sha256"
    identifier_format = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"

    idp_metadata {
      entity_id = "https://okta.example.com"
      metadata  = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" entityID=\"https://okta.example.com\"><IDPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"><SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect\" Location=\"https://okta.example.com/app/sso/saml\"/></IDPSSODescriptor></EntityDescriptor>"

      single_sign_on_service {
        binding  = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
        location = "https://okta.example.com/app/sso/saml"
      }
    }

    sp_metadata {
      entity_id = "https://archestra.example.com"
      metadata  = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" entityID=\"https://archestra.example.com\"><SPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"><AssertionConsumerService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://archestra.example.com/api/auth/sso/saml2/sp/acs/OktaSAML\" index=\"0\" isDefault=\"true\"/></SPSSODescriptor></EntityDescriptor>"
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
