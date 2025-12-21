package provider

import (
	"context"
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

// Unit tests for provider.
func TestProviderNew(t *testing.T) {
	provider := New("test")()
	if provider == nil {
		t.Fatal("Expected provider to be non-nil")
	}
}

func TestProviderRegistrationCounts(t *testing.T) {
	p, ok := New("test")().(*ArchestraProvider)
	if !ok {
		t.Fatal("Expected ArchestraProvider type")
	}

	t.Run("Resources_RegistrationCount", func(t *testing.T) {
		resources := p.Resources(context.Background())
		if len(resources) != 13 {
			t.Fatalf("Expected 13 resources to be registered, got %d", len(resources))
		}
	})

	t.Run("DataSources_RegistrationCount", func(t *testing.T) {
		dataSources := p.DataSources(context.Background())
		if len(dataSources) != 5 {
			t.Fatalf("Expected 5 data sources to be registered, got %d", len(dataSources))
		}
	})
}
