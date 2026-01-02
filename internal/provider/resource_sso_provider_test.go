package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var (
	ssoSensitiveFields = []string{
		"oidc_config.0.client_secret",
		"saml_config.0.cert",
		"saml_config.0.decryption_pvk",
		"saml_config.0.private_key",
		"saml_config.0.idp_metadata.0.cert",
		"saml_config.0.idp_metadata.0.enc_private_key",
		"saml_config.0.idp_metadata.0.enc_private_key_pass",
		"saml_config.0.idp_metadata.0.private_key",
		"saml_config.0.idp_metadata.0.private_key_pass",
		"saml_config.0.sp_metadata.0.enc_private_key",
		"saml_config.0.sp_metadata.0.enc_private_key_pass",
		"saml_config.0.sp_metadata.0.private_key",
		"saml_config.0.sp_metadata.0.private_key_pass",
	}

	// ssoAPIDefaultFields contains fields where API returns defaults that may differ from config.
	ssoAPIDefaultFields = []string{
		"oidc_config.0.override_user_info",
		"oidc_config.0.mapping.0.id",
		"oidc_config.override_user_info",
		"oidc_config.mapping.id",
		"saml_config.0.mapping.0.id",
		"saml_config.mapping.id",
	}

	// ssoTeamSyncFields contains team sync config fields (optional, may not be returned).
	ssoTeamSyncFields = []string{
		"team_sync_config.0.%",
		"team_sync_config.0.enabled",
		"team_sync_config.0.groups_expression",
		"team_sync_config.%",
		"team_sync_config.enabled",
		"team_sync_config.groups_expression",
	}

	// ssoImportStateVerifyIgnore is the combined list of all fields to ignore during import verification.
	ssoImportStateVerifyIgnore = append(append(append([]string{}, ssoSensitiveFields...), ssoAPIDefaultFields...), ssoTeamSyncFields...)
)

func TestAccSsoProviderResource_oidc(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSsoProviderOIDCConfig("test-oidc", "example.com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_sso_provider.test", tfjsonpath.New("provider_id"), knownvalue.StringExact("test-oidc")),
					statecheck.ExpectKnownValue("archestra_sso_provider.test", tfjsonpath.New("domain"), knownvalue.StringExact("example.com")),
				},
			},
			{
				ResourceName:            "archestra_sso_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: ssoImportStateVerifyIgnore,
			},
			{
				Config: testAccSsoProviderOIDCConfigUpdated("test-oidc", "example.org"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_sso_provider.test", tfjsonpath.New("domain"), knownvalue.StringExact("example.org")),
				},
			},
		},
	})
}

func testAccSsoProviderOIDCConfig(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_sso_provider" "test" {
  provider_id = %q
  domain      = %q
  issuer      = "https://accounts.example.com"

  oidc_config {
    issuer              = "https://accounts.example.com"
    discovery_endpoint  = "https://accounts.example.com/.well-known/openid-configuration"
    client_id           = "terraform-client"
    client_secret       = "terraform-secret"
    scopes              = ["openid", "email", "profile"]
    pkce                = true
    override_user_info  = false
    token_endpoint_authentication = "client_secret_post"

    mapping {
      email = "email"
      name  = "name"
    }
  }

  role_mapping {
    default_role = "member"
    rules = [{
      expression = "true"
      role       = "admin"
    }]
  }

  team_sync_config {
    enabled           = true
    groups_expression = "{{#each groups}}{{this}},{{/each}}"
  }
}
`, providerID, domain)
}

func testAccSsoProviderOIDCConfigUpdated(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_sso_provider" "test" {
  provider_id = %q
  domain      = %q
  issuer      = "https://accounts.example.com"

  oidc_config {
    issuer              = "https://accounts.example.com"
    discovery_endpoint  = "https://accounts.example.com/.well-known/openid-configuration"
    client_id           = "terraform-client"
    client_secret       = "terraform-secret"
    scopes              = ["openid", "email"]
    pkce                = false
    token_endpoint_authentication = "client_secret_basic"
  }

  role_mapping {
    default_role = "member"
  }
}
`, providerID, domain)
}

func TestAccSsoProviderResource_saml(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSsoProviderSAMLConfig("test-saml", "example.com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_sso_provider.test", tfjsonpath.New("provider_id"), knownvalue.StringExact("test-saml")),
					statecheck.ExpectKnownValue("archestra_sso_provider.test", tfjsonpath.New("domain"), knownvalue.StringExact("example.com")),
				},
			},
			{
				ResourceName:            "archestra_sso_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: ssoImportStateVerifyIgnore,
			},
			{
				Config: testAccSsoProviderSAMLConfigUpdated("test-saml", "example.org"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_sso_provider.test", tfjsonpath.New("domain"), knownvalue.StringExact("example.org")),
				},
			},
		},
	})
}

func testAccSsoProviderSAMLConfig(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_sso_provider" "test" {
  provider_id = %q
  domain      = %q
  issuer      = "https://example.com"

  saml_config {
    issuer       = "https://example.com"
    entry_point  = "https://idp.example.com/saml/sso"
    callback_url = "https://archestra.example.com/api/auth/sso/saml2/sp/acs/%s"
    cert         = "-----BEGIN CERTIFICATE-----\nMIICiTCCAg+gAwIBAgIJAJ8l4HnPq7F8MAOGA1UEBhMCVVMxCzAJBgNVBAgTAkNB\nMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRowGAYDVQQKExFPcGVuQU1QIFNhbXBs\nZSBDb21wYW55IENBMRYwFAYDVQQDEx1vcGVuYW1wLmV4YW1wbGUuY29tMB4XDTE0\nMDgxOTE2MjQyNVoXDTIyMDgxODE2MjQyNVowdTELMAkGA1UEBhMCVVMxCzAJBgNV\nBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRowGAYDVQQKExFPcGVuQU1Q\nIFNhbXBsZSBDb21wYW55IENBMRYwFAYDVQQDEx1vcGVuYW1wLmV4YW1wbGUuY29t\nMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANgOqCbLsKv5CF+vGmJ9Vq5PJKKuiU8+\nLpqtHKHC9q3mRWxHF8dlE8j9D6Kz+N+CK+qGzFjWNBT3UVFzU5GJUYCAwEAAaNQ\nME4wHQYDVR0OBBYEFG7CJM9GjHn7Lqt8kJc8W5proUwWMB8GA1UdIwQYMBaAFG7C\nJM9GjHn7Lqt8kJc8W5proUwWMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQAD\nggEBABYIUUUeWDJ+wZF0lZ+mJnRnGZpXL2fKe3+KGjNM8xJfPf2YvqU4mgdMxgJn\n-----END CERTIFICATE-----"

    audience         = "https://archestra.example.com"
    digest_algorithm = "sha256"
    identifier_format = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"

    idp_metadata {
      entity_id = "https://idp.example.com"
      metadata  = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" entityID=\"https://idp.example.com\"><IDPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"><SingleSignOnService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect\" Location=\"https://idp.example.com/saml/sso\"/></IDPSSODescriptor></EntityDescriptor>"

      single_sign_on_service {
        binding  = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
        location = "https://idp.example.com/saml/sso"
      }
    }

    sp_metadata {
      entity_id = "https://archestra.example.com"
      metadata  = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><EntityDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\" entityID=\"https://archestra.example.com\"><SPSSODescriptor protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"><AssertionConsumerService Binding=\"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST\" Location=\"https://archestra.example.com/api/auth/sso/saml2/sp/acs/%s\" index=\"0\" isDefault=\"true\"/></SPSSODescriptor></EntityDescriptor>"
    }

    mapping {
      email      = "email"
      first_name = "firstName"
      last_name  = "lastName"
      name       = "name"
    }
  }

  role_mapping {
    default_role = "member"
    strict_mode  = true
    rules = [{
      expression = "{{#includes groups \"saml-admins\"}}true{{/includes}}"
      role       = "admin"
    }]
  }

  team_sync_config {
    enabled           = true
    groups_expression = "{{#each groups}}{{this}},{{/each}}"
  }
}
`, providerID, domain, providerID, providerID)
}

func testAccSsoProviderSAMLConfigUpdated(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_sso_provider" "test" {
  provider_id = %q
  domain      = %q
  issuer      = "https://example.com"

  saml_config {
    issuer       = "https://example.com"
    entry_point  = "https://idp.example.com/saml/sso"
    callback_url = "https://archestra.example.com/api/auth/sso/saml2/sp/acs/%s"
    cert         = "-----BEGIN CERTIFICATE-----\nMIICiTCCAg+gAwIBAgIJAJ8l4HnPq7F8MAOGA1UEBhMCVVMxCzAJBgNVBAgTAkNB\nMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRowGAYDVQQKExFPcGVuQU1QIFNhbXBs\nZSBDb21wYW55IENBMRYwFAYDVQQDEx1vcGVuYW1wLmV4YW1wbGUuY29tMB4XDTE0\nMDgxOTE2MjQyNVoXDTIyMDgxODE2MjQyNVowdTELMAkGA1UEBhMCVVMxCzAJBgNV\nBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRowGAYDVQQKExFPcGVuQU1Q\nIFNhbXBsZSBDb21wYW55IENBMRYwFAYDVQQDEx1vcGVuYW1wLmV4YW1wbGUuY29t\nMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANgOqCbLsKv5CF+vGmJ9Vq5PJKKuiU8+\nLpqtHKHC9q3mRWxHF8dlE8j9D6Kz+N+CK+qGzFjWNBT3UVFzU5GJUYCAwEAAaNQ\nME4wHQYDVR0OBBYEFG7CJM9GjHn7Lqt8kJc8W5proUwWMB8GA1UdIwQYMBaAFG7C\nJM9GjHn7Lqt8kJc8W5proUwWMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQAD\nggEBABYIUUUeWDJ+wZF0lZ+mJnRnGZpXL2fKe3+KGjNM8xJfPf2YvqU4mgdMxgJn\n-----END CERTIFICATE-----"

    want_assertions_signed = true

    mapping {
      email = "email"
      name  = "name"
    }
  }

  role_mapping {
    default_role = "viewer"
  }
}
`, providerID, domain, providerID)
}
