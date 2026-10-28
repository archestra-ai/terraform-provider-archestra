package client

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
