package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// We do NOT define testAccProtoV6ProviderFactories here because
// it is already defined in provider_test.go

func TestAccSSOProviderResource_Mocked(t *testing.T) {
	// 1. Setup Mock Server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle POST (Create) and GET (Read)
		if r.Method == http.MethodPost || r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
                "id": "sso-12345",
                "providerId": "google",
                "domain": "example.com",
                "issuer": "https://accounts.google.com",
                "oidcConfig": {
                    "clientId": "test-client",
                    "clientSecret": "test-secret"
                }
            }`))
			return
		}

		// Handle DELETE
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// 2. Define Config
	// IMPORTANT: We use 'base_url' because that is what your provider.go expects.
	testConfig := fmt.Sprintf(`
        provider "archestra" {
            base_url = "%s"
            api_key  = "mock-key"
        }

        resource "archestra_sso_provider" "test" {
            provider_id = "google"
            domain      = "example.com"
            
            oidc_config = {
                client_id     = "test-client"
                client_secret = "test-secret"
            }
        }
    `, mockServer.URL)

	// 3. Run Test
	resource.Test(t, resource.TestCase{
		// Use the factory defined in provider_test.go
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_sso_provider.test", "id", "sso-12345"),
					resource.TestCheckResourceAttr("archestra_sso_provider.test", "provider_id", "google"),
					resource.TestCheckResourceAttr("archestra_sso_provider.test", "domain", "example.com"),
				),
			},
		},
	})
}
