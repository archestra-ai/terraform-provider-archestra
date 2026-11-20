package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWorkloadIdentityTokenSourceExchangesAndCaches(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != workloadIdentityTokenPath || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{
			"grant_type":         tokenExchangeGrantType,
			"subject_token_type": jwtTokenType,
			"subject_token":      "oidc-1",
			"service_account_id": "sa-1",
		}
		for k, v := range want {
			if got := r.PostForm.Get(k); got != v {
				t.Errorf("%s = %q, want %q", k, got, v)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"archestra-token","issued_token_type":"urn:ietf:params:oauth:token-type:access_token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	now := time.Unix(1_000_000, 0)
	source := &workloadIdentityTokenSource{httpClient: server.Client(), baseURL: server.URL + "/", serviceAccountID: "sa-1", oidcToken: "oidc-1", now: func() time.Time { return now }}

	for range 3 {
		token, err := source.Token(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if token != "archestra-token" {
			t.Fatalf("token = %q", token)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("exchanged %d times, want 1 while the token is fresh", calls.Load())
	}

	now = now.Add(time.Hour - workloadIdentityRefreshMargin)
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("exchanged %d times, want a refresh inside the expiry margin", calls.Load())
	}
}

func TestWorkloadIdentityTokenSourceRereadsRotatedTokenFile(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		_ = r.ParseForm()
		seen = append(seen, r.PostForm.Get("subject_token"))
		_, _ = w.Write([]byte(`{"access_token":"t","expires_in":60}`))
	}))
	defer server.Close()

	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(0, 0)
	source := &workloadIdentityTokenSource{httpClient: server.Client(), baseURL: server.URL, serviceAccountID: "sa", oidcTokenFile: file, now: func() time.Time { return now }}
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A 60s token is always inside the refresh margin, so every call exchanges.
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "first,second" {
		t.Fatalf("subject tokens = %v, want the trimmed file contents, re-read per exchange", seen)
	}
}

func TestWorkloadIdentityTokenExchangeErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"refused", http.StatusBadRequest, `{"error":"invalid_grant","error_description":"No federated credential matches."}`, "invalid_grant): No federated credential matches."},
		{"unsupported server", http.StatusNotFound, `{}`, "does not support workload identity token exchange"},
		{"no token", http.StatusOK, `{"expires_in":3600}`, "no access token"},
		{"opaque failure", http.StatusBadGateway, `bad gateway`, "HTTP 502"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			source := &workloadIdentityTokenSource{httpClient: server.Client(), baseURL: server.URL, serviceAccountID: "sa", oidcToken: "x", now: time.Now}
			_, err := source.Token(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestWorkloadIdentityTokenSourceMissingFile(t *testing.T) {
	source := &workloadIdentityTokenSource{serviceAccountID: "sa", oidcTokenFile: filepath.Join(t.TempDir(), "missing"), now: time.Now}
	if _, err := source.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "cannot read the OIDC token file") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateProviderAuth(t *testing.T) {
	cases := []struct {
		name     string
		apiKey   string
		workload workloadIdentityConfig
		summary  string
	}{
		{"api key", "arch_x", workloadIdentityConfig{}, ""},
		{"workload identity file", "", workloadIdentityConfig{serviceAccountID: "sa", oidcTokenFile: "/t"}, ""},
		{"workload identity inline", "", workloadIdentityConfig{serviceAccountID: "sa", oidcToken: "jwt"}, ""},
		{"nothing", "", workloadIdentityConfig{}, "Missing Archestra API Key"},
		{"both credentials", "arch_x", workloadIdentityConfig{serviceAccountID: "sa", oidcTokenFile: "/t"}, "Conflicting Archestra credentials"},
		{"no token", "", workloadIdentityConfig{serviceAccountID: "sa"}, "Missing OIDC token"},
		{"token without account", "", workloadIdentityConfig{oidcToken: "jwt"}, "Missing Archestra service account ID"},
		{"two token sources", "", workloadIdentityConfig{serviceAccountID: "sa", oidcToken: "jwt", oidcTokenFile: "/t"}, "Conflicting OIDC token settings"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := validateProviderAuth(tc.apiKey, tc.workload)
			if tc.summary == "" {
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags)
				}
				return
			}
			if !diags.HasError() || diags.Errors()[0].Summary() != tc.summary {
				t.Fatalf("diagnostics = %v, want %q", diags, tc.summary)
			}
		})
	}
}

func TestResolveWorkloadIdentityConfigPrefersAttributes(t *testing.T) {
	t.Setenv("ARCHESTRA_SERVICE_ACCOUNT_ID", "from-env")
	t.Setenv("ARCHESTRA_OIDC_TOKEN_FILE", "/env/token")
	t.Setenv("ARCHESTRA_OIDC_TOKEN", "")
	got := resolveWorkloadIdentityConfig(ArchestraProviderModel{ServiceAccountID: types.StringValue("from-config")})
	if got.serviceAccountID != "from-config" || got.oidcTokenFile != "/env/token" || got.oidcToken != "" {
		t.Fatalf("resolved = %+v", got)
	}
}
