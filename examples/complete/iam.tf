resource "archestra_identity_provider" "oidc" {
  provider_id = "DemoOIDC"
  domain      = var.oidc_domain
  issuer      = var.oidc_issuer

  oidc_config {
    issuer             = var.oidc_issuer
    discovery_endpoint = var.oidc_discovery_endpoint
    client_id          = var.oidc_client_id
    client_secret      = var.oidc_client_secret
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
    rules = [
      {
        expression = "{{#includes groups \"archestra-admins\"}}true{{/includes}}"
        role       = "admin"
      },
      {
        expression = "{{#includes groups \"archestra-readonly\"}}true{{/includes}}"
        role       = "viewer"
      },
    ]
  }

  team_sync_config {
    enabled           = true
    groups_expression = "{{#each groups}}{{this}},{{/each}}"
  }
}

resource "archestra_identity_provider" "saml" {
  provider_id = "DemoSAML"
  domain      = "demo-saml.example.com"
  issuer      = "https://saml.example.com"

  saml_config {
    issuer            = "https://saml.example.com"
    entry_point       = "https://okta.example.com/app/sso/saml"
    callback_url      = "https://archestra.example.com/api/auth/sso/saml2/sp/acs/DemoSAML"
    cert              = var.saml_cert
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
      metadata  = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" entityID=\"https://archestra.example.com\"><SPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"><AssertionConsumerService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://archestra.example.com/api/auth/sso/saml2/sp/acs/DemoSAML\" index=\"0\" isDefault=\"true\"/></SPSSODescriptor></EntityDescriptor>"
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
    rules = [
      {
        expression = "{{#includes groups \"saml-admins\"}}true{{/includes}}"
        role       = "admin"
      },
    ]
  }

  team_sync_config {
    enabled           = true
    groups_expression = "{{#each groups}}{{this}},{{/each}}"
  }
}

resource "archestra_team" "engineering" {
  name        = "Engineering"
  description = "Engineering team for production systems"
}

# Support team with TOON tool-result compression enabled. The team-level flag
# overrides the org default from `archestra_organization_settings`.
resource "archestra_team" "support" {
  name                         = "Support"
  description                  = "Customer support team"
  convert_tool_results_to_toon = true
}

resource "archestra_team_external_group" "engineers_oidc" {
  team_id           = archestra_team.engineering.id
  external_group_id = "engineering"
}
