package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
//
//nolint:unused // Will be used by resource/datasource tests
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"archestra": providerserver.NewProtocol6WithError(New("test")()),
}

//nolint:unused // Will be used by resource/datasource tests
func testAccPreCheck(t *testing.T) {
	// Check for required environment variables for acceptance tests
	if v := os.Getenv("ARCHESTRA_API_KEY"); v == "" {
		t.Fatal("ARCHESTRA_API_KEY must be set for acceptance tests")
	}
	if v := os.Getenv("ARCHESTRA_BASE_URL"); v == "" {
		t.Fatal("ARCHESTRA_BASE_URL must be set for acceptance tests")
	}
}

//nolint:unused // Used by vault-ref-dependent tests
func testAccRequireByosEnabled(t *testing.T) {
	if os.Getenv("ARCHESTRA_READONLY_VAULT_ENABLED") != "true" {
		t.Fatal("requires ARCHESTRA_READONLY_VAULT_ENABLED=true and a backend with ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT + EE license")
	}
}

// Unit tests for provider.
func TestProviderNew(t *testing.T) {
	provider := New("test")()
	if provider == nil {
		t.Fatal("Expected provider to be non-nil")
	}
}
