package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelPoolWritePreflightDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"missing route", 404, "sensitive-response-body", "returned HTTP 404"},
		{"unauthorized", 401, "sensitive-response-body", "returned HTTP 401"},
		{"forbidden", 403, "sensitive-response-body", "returned HTTP 403"},
		{"unavailable", 503, "sensitive-response-body", "returned HTTP 503"},
		{"frontend HTML", 200, "<html>sensitive-response-body</html>", "returned an invalid JSON response"},
		{"wrong shape", 200, `{"paths":{"sensitive-response-body":42}}`, "returned an invalid JSON response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet || r.URL.Path != "/prefix/openapi.json" {
					t.Errorf("unexpected request after failed preflight: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "sensitive-api-key" {
					t.Error("preflight lost configured authentication")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			api, err := NewClientWithResponses(strings.Replace(server.URL, "://", "://sensitive-user:sensitive-password@", 1)+"/prefix/", WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
				r.Header.Set("Authorization", "sensitive-api-key")
				r.URL.RawQuery = "token=sensitive-query"
				r.URL.Fragment = "sensitive-fragment"
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			pool, err := NewModelPoolClient(api)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"", "11111111-1111-4111-8111-111111111111"} {
				_, err := pool.Write(t.Context(), id, bytes.NewBufferString(`{"name":"synthetic","members":[]}`))
				if err == nil || !strings.Contains(err.Error(), "OpenAPI preflight GET "+server.URL+"/prefix/openapi.json "+tc.want) {
					t.Fatalf("missing preflight URL or failure reason: %v", err)
				}
				if strings.Contains(err.Error(), "sensitive-") || strings.Contains(err.Error(), "model pool get") {
					t.Fatalf("unsafe or misleading preflight diagnostic: %v", err)
				}
			}
			if requests != 2 {
				t.Fatalf("got %d requests, want one preflight per write", requests)
			}
		})
	}
}

type modelPoolFailingTransport struct{}

func (modelPoolFailingTransport) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("sensitive-transport-details")
}

func TestModelPoolPreflightTransportDiagnostic(t *testing.T) {
	api, err := NewClientWithResponses("https://sensitive-user:sensitive-password@example.test/prefix/", WithHTTPClient(modelPoolFailingTransport{}))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := NewModelPoolClient(api)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Write(t.Context(), "", bytes.NewBufferString(`{}`))
	if err == nil || !strings.Contains(err.Error(), "OpenAPI preflight GET https://example.test/prefix/openapi.json failed") || strings.Contains(err.Error(), "sensitive-") {
		t.Fatalf("unsafe or missing preflight transport diagnostic: %v", err)
	}
}

func TestModelPoolWriteRejectsOldGrantContractBeforeMutation(t *testing.T) {
	for _, extra := range []string{`,"virtualApiKeyIds":{}`, ""} {
		t.Run(fmt.Sprint(extra != ""), func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/openapi.json" {
					op := fmt.Sprintf(`{"requestBody":{"content":{"application/json":{"schema":{"properties":{"name":{},"members":{}%s}}}}}}`, extra)
					fmt.Fprintf(w, `{"paths":{"/api/model-pools":{"post":%s},"/api/model-pools/{id}":{"put":%s}}}`, op, op)
					return
				}
				writes++
				fmt.Fprint(w, `{"id":"11111111-1111-4111-8111-111111111111"}`)
			}))
			defer server.Close()
			api, err := NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			pool, err := NewModelPoolClient(api)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"", "11111111-1111-4111-8111-111111111111"} {
				_, err = pool.Write(t.Context(), id, bytes.NewBufferString(`{"name":"synthetic","members":[]}`))
				if (err != nil) != (extra != "") {
					t.Fatalf("unexpected compatibility result: %v", err)
				}
			}
			expected := 2
			if extra != "" {
				expected = 0
			}
			if writes != expected {
				t.Fatalf("writes=%d expected=%d", writes, expected)
			}
		})
	}
}
