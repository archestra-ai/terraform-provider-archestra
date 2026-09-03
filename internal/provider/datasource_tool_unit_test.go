package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

func TestGetToolsReadsEveryPaginatedPage(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit = %q, want 100", got)
		}

		w.Header().Set("Content-Type", "application/json")
		switch offset := r.URL.Query().Get("offset"); offset {
		case "0":
			fmt.Fprint(w, `{"data":[{"id":"first-id","name":"first"}],"pagination":{"hasNext":true}}`)
		case "100":
			fmt.Fprint(w, `{"data":[{"id":"second-id","name":"second","description":"second tool"}],"pagination":{"hasNext":false}}`)
		default:
			t.Errorf("unexpected offset %q", offset)
			http.Error(w, "unexpected offset", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	apiClient, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	tools, err := getTools(context.Background(), apiClient)
	if err != nil {
		t.Fatalf("get tools: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(tools))
	}
	if tools[1].Name != "second" || tools[1].Description == nil || *tools[1].Description != "second tool" {
		t.Fatalf("unexpected second tool: %#v", tools[1])
	}
}

func TestGetToolsSupportsUnpaginatedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"tool-id","name":"tool"}]`)
	}))
	t.Cleanup(server.Close)

	apiClient, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	tools, err := getTools(context.Background(), apiClient)
	if err != nil {
		t.Fatalf("get tools: %v", err)
	}
	if len(tools) != 1 || tools[0].ID != "tool-id" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

func TestDecodeToolPageRejectsInvalidResponse(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`not-json`, `{}`, `{"pagination":{"hasNext":false}}`} {
		if _, _, _, err := decodeToolPage([]byte(body)); err == nil {
			t.Errorf("decodeToolPage(%q) returned no error", body)
		}
	}
}
