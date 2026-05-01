package provider

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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

func TestResolveHTTPTimeout(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{name: "unset uses default", raw: "", want: defaultHTTPTimeout},
		{name: "valid override", raw: "45s", want: 45 * time.Second},
		{name: "minutes parse", raw: "5m", want: 5 * time.Minute},
		{name: "garbage rejected", raw: "not-a-duration", wantErr: true},
		{name: "zero rejected", raw: "0", wantErr: true},
		{name: "zero seconds rejected", raw: "0s", wantErr: true},
		{name: "negative rejected", raw: "-1m", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveHTTPTimeout(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for raw=%q, got nil (returned %v)", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for raw=%q: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("raw=%q: got %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNewHTTPTransport_PreservesDefaults(t *testing.T) {
	tr := newHTTPTransport()
	if tr.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 10s", tr.TLSHandshakeTimeout)
	}
	// Cloning DefaultTransport (vs. zeroing one) preserves Proxy lookup —
	// regression check for someone replacing .Clone() with &http.Transport{}.
	if tr.Proxy == nil {
		t.Error("Proxy is nil; HTTPS_PROXY would stop working")
	}
	if tr.DialContext == nil {
		t.Error("DialContext is nil; default dial timeouts would stop applying")
	}
}

func TestBuildHTTPClient_WiredEndToEnd(t *testing.T) {
	// Proves the timeout actually fires; the other tests only check shape.
	t.Setenv("ARCHESTRA_HTTP_TIMEOUT", "50ms")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	c, err := buildHTTPClient()
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}

	start := time.Now()
	resp, err := c.Do(req)
	elapsed := time.Since(start)

	// http.Client may return both a non-nil resp and a non-nil err; close
	// either to avoid leaking the connection regardless of which path runs.
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("request took %v; expected ~50ms timeout to fire", elapsed)
	}
}
