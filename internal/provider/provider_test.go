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

// Unit tests for provider.
func TestProviderNew(t *testing.T) {
	provider := New("test")()
	if provider == nil {
		t.Fatal("Expected provider to be non-nil")
	}
}

func TestProviderResources_RegistrationCount(t *testing.T) {
	provider, ok := New("test")().(*ArchestraProvider)
	if !ok {
		t.Fatal("Expected ArchestraProvider type")
	}
	resources := provider.Resources(t.Context())

	// We expect 9 resources to be registered
	expectedCount := 9 // agent, mcp_server, mcp_server_installation, team, tool_invocation_policy, trusted_data_policy, token_price, limit, optimization_rule
	if len(resources) != expectedCount {
		t.Errorf("Expected %d resources to be registered, got %d", expectedCount, len(resources))
	}
}

func TestProviderDataSources_RegistrationCount(t *testing.T) {
	provider, ok := New("test")().(*ArchestraProvider)
	if !ok {
		t.Fatal("Expected ArchestraProvider type")
	}
	dataSources := provider.DataSources(t.Context())

	// We expect 4 data sources to be registered
	expectedCount := 4 // team, agent_tool, mcp_server_tool, token_prices
	if len(dataSources) != expectedCount {
		t.Errorf("Expected %d data sources to be registered, got %d", expectedCount, len(dataSources))
	}
}
