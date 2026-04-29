package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var (
	identityProviderSensitiveFields = []string{
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

	// identityProviderAPIDefaultFields used to ignore fields where the
	// API echoed null instead of the schema default (override_user_info,
	// mapping.id). The flatteners now materialize the declared default
	// when the API omits the field, so import round-trips cleanly. Kept
	// as an empty slice so the append at identityProviderImportStateVerifyIgnore
	// stays a no-op without churning that line.
	identityProviderAPIDefaultFields = []string{}

	// identityProviderTeamSyncFields contains team sync config fields (optional, may not be returned).
	identityProviderTeamSyncFields = []string{
		"team_sync_config.0.%",
		"team_sync_config.0.enabled",
		"team_sync_config.0.groups_expression",
		"team_sync_config.%",
		"team_sync_config.enabled",
		"team_sync_config.groups_expression",
	}

	// identityProviderImportStateVerifyIgnore is the combined list of all fields to ignore during import verification.
	identityProviderImportStateVerifyIgnore = append(append(append([]string{}, identityProviderSensitiveFields...), identityProviderAPIDefaultFields...), identityProviderTeamSyncFields...)
)

func TestAccIdentityProviderResource_oidc(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdentityProviderOIDCConfig("test-oidc", "example.com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_identity_provider.test", tfjsonpath.New("provider_id"), knownvalue.StringExact("test-oidc")),
					statecheck.ExpectKnownValue("archestra_identity_provider.test", tfjsonpath.New("domain"), knownvalue.StringExact("example.com")),
				},
			},
			{
				ResourceName:            "archestra_identity_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: identityProviderImportStateVerifyIgnore,
			},
			{
				Config: testAccIdentityProviderOIDCConfigUpdated("test-oidc", "example.org"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_identity_provider.test", tfjsonpath.New("domain"), knownvalue.StringExact("example.org")),
				},
			},
		},
	})
}

func testAccIdentityProviderOIDCConfig(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_identity_provider" "test" {
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
    skip_discovery      = true
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

func testAccIdentityProviderOIDCConfigUpdated(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_identity_provider" "test" {
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
    skip_discovery      = true
    token_endpoint_authentication = "client_secret_basic"
  }

  role_mapping {
    default_role = "member"
  }
}
`, providerID, domain)
}

func TestAccIdentityProviderResource_oidcWithEnterpriseCredentials(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdentityProviderOIDCWithEnterpriseCredentialsConfig("test-enterprise", "enterprise.example.com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_identity_provider.test",
						tfjsonpath.New("provider_id"),
						knownvalue.StringExact("test-enterprise"),
					),
					statecheck.ExpectKnownValue(
						"archestra_identity_provider.test",
						tfjsonpath.New("domain"),
						knownvalue.StringExact("enterprise.example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_identity_provider.test",
						tfjsonpath.New("oidc_config").AtMapKey("enterprise_managed_credentials").AtMapKey("exchange_strategy"),
						knownvalue.StringExact("rfc8693"),
					),
					statecheck.ExpectKnownValue(
						"archestra_identity_provider.test",
						tfjsonpath.New("oidc_config").AtMapKey("enterprise_managed_credentials").AtMapKey("client_id"),
						knownvalue.StringExact("downstream-client"),
					),
				},
			},
			{
				ResourceName:            "archestra_identity_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: identityProviderImportStateVerifyIgnore,
			},
		},
	})
}

// TestAccIdentityProviderResource_oidcExchangeStrategies exercises every enum
// value of `enterprise_managed_credentials.exchange_strategy` via in-place
// updates. Catches regressions if the backend renames the enum again.
func TestAccIdentityProviderResource_oidcExchangeStrategies(t *testing.T) {
	strategies := []string{"rfc8693", "okta_managed", "entra_obo"}
	steps := make([]resource.TestStep, 0, len(strategies))
	for _, s := range strategies {
		steps = append(steps, resource.TestStep{
			Config: testAccIdentityProviderOIDCExchangeStrategyConfig("test-exchange-strategy", "exchange.example.com", s),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(
					"archestra_identity_provider.test",
					tfjsonpath.New("oidc_config").AtMapKey("enterprise_managed_credentials").AtMapKey("exchange_strategy"),
					knownvalue.StringExact(s),
				),
			},
		})
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

func testAccIdentityProviderOIDCExchangeStrategyConfig(providerID, domain, strategy string) string {
	return fmt.Sprintf(`
resource "archestra_identity_provider" "test" {
  provider_id = %q
  domain      = %q
  issuer      = "https://%[3]s.example.com"

  oidc_config {
    issuer             = "https://%[3]s.example.com"
    discovery_endpoint = "https://%[3]s.example.com/.well-known/openid-configuration"
    client_id          = "enterprise-client"
    client_secret      = "enterprise-secret"
    pkce               = true
    skip_discovery     = true

    enterprise_managed_credentials {
      exchange_strategy             = %[4]q
      client_id                     = "downstream-client"
      client_secret                 = "downstream-secret"
      token_endpoint                = "https://%[3]s.example.com/oauth/token"
      token_endpoint_authentication = "client_secret_post"
    }
  }
}
`, providerID, domain, strategy, strategy)
}

func testAccIdentityProviderOIDCWithEnterpriseCredentialsConfig(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_identity_provider" "test" {
  provider_id = %q
  domain      = %q
  issuer      = "https://enterprise.example.com"

  oidc_config {
    issuer             = "https://enterprise.example.com"
    discovery_endpoint = "https://enterprise.example.com/.well-known/openid-configuration"
    client_id          = "enterprise-client"
    client_secret      = "enterprise-secret"
    pkce               = true
    skip_discovery     = true
    token_endpoint_authentication = "client_secret_post"

    enterprise_managed_credentials {
      exchange_strategy            = "rfc8693"
      client_id                    = "downstream-client"
      client_secret                = "downstream-secret"
      token_endpoint               = "https://enterprise.example.com/oauth/token"
      token_endpoint_authentication = "client_secret_post"
    }
  }

  role_mapping {
    default_role = "member"
  }
}
`, providerID, domain)
}

func TestAccIdentityProviderResource_saml(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdentityProviderSAMLConfig("test-saml", "example.com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_identity_provider.test", tfjsonpath.New("provider_id"), knownvalue.StringExact("test-saml")),
					statecheck.ExpectKnownValue("archestra_identity_provider.test", tfjsonpath.New("domain"), knownvalue.StringExact("example.com")),
					statecheck.ExpectKnownValue(
						"archestra_identity_provider.test",
						tfjsonpath.New("saml_config").AtMapKey("additional_params"),
						knownvalue.StringExact(`{"Custom":"value","ForceAuthn":true,"MaxAge":3600}`),
					),
				},
			},
			{
				ResourceName:            "archestra_identity_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: identityProviderImportStateVerifyIgnore,
			},
			{
				Config: testAccIdentityProviderSAMLConfigUpdated("test-saml", "example.org"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_identity_provider.test", tfjsonpath.New("domain"), knownvalue.StringExact("example.org")),
				},
			},
		},
	})
}

func testAccIdentityProviderSAMLConfig(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_identity_provider" "test" {
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

    additional_params = jsonencode({
      ForceAuthn = true
      MaxAge     = 3600
      Custom     = "value"
    })

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

func testAccIdentityProviderSAMLConfigUpdated(providerID, domain string) string {
	return fmt.Sprintf(`
resource "archestra_identity_provider" "test" {
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

// TestAccIdentityProviderResource_BothConfigsSet pins the ValidateConfig
// XOR check: oidc_config and saml_config must be mutually exclusive at
// plan time, not apply time.
func TestAccIdentityProviderResource_BothConfigsSet(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_identity_provider" "test" {
  provider_id = "tf-acc-idp-both"
  domain      = "example.com"
  issuer      = "https://example.com"

  oidc_config {
    issuer        = "https://example.com"
    client_id     = "client-id"
    client_secret = "client-secret"
  }

  saml_config {
    issuer       = "https://example.com"
    entry_point  = "https://idp.example.com/saml/sso"
    callback_url = "https://archestra.example.com/cb"
    cert         = "x"
  }
}
`,
				ExpectError: regexp.MustCompile(`only one of oidc_config or saml_config`),
			},
		},
	})
}

// TestAccIdentityProviderResource_NeitherConfigSet pins the other arm of
// the XOR — at least one config block must be present.
func TestAccIdentityProviderResource_NeitherConfigSet(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_identity_provider" "test" {
  provider_id = "tf-acc-idp-neither"
  domain      = "example.com"
  issuer      = "https://example.com"
}
`,
				ExpectError: regexp.MustCompile(`exactly one of oidc_config or saml_config`),
			},
		},
	})
}
