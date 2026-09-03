package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

func TestFindToolByNameUsesSearchAndReadsEveryPage(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/tools/with-assignments" {
			t.Errorf("path = %q, want /api/tools/with-assignments", r.URL.Path)
		}
		if got := r.URL.Query().Get("search"); got != "target" {
			t.Errorf("search = %q, want target", got)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit = %q, want 100", got)
		}

		w.Header().Set("Content-Type", "application/json")
		switch offset := r.URL.Query().Get("offset"); offset {
		case "0":
			fmt.Fprint(w, `{"data":[{"id":"similar-id","name":"target-helper"}],"pagination":{"hasNext":true}}`)
		case "100":
			fmt.Fprint(w, `{"data":[{"id":"target-id","name":"target","description":"target tool"}],"pagination":{"hasNext":false}}`)
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

	tool, found, err := findToolByName(context.Background(), apiClient, "target")
	if err != nil {
		t.Fatalf("find tool: %v", err)
	}
	if !found {
		t.Fatal("tool not found")
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if tool.ID != "target-id" || tool.Description == nil || *tool.Description != "target tool" {
		t.Fatalf("unexpected tool: %#v", tool)
	}
}

func TestFindToolByNameReturnsNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"pagination":{"hasNext":false}}`)
	}))
	t.Cleanup(server.Close)

	apiClient, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	_, found, err := findToolByName(context.Background(), apiClient, "missing")
	if err != nil {
		t.Fatalf("find tool: %v", err)
	}
	if found {
		t.Fatal("unexpected tool match")
	}
}

func TestFindToolByNameRejectsUnexpectedStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	apiClient, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	if _, _, err := findToolByName(context.Background(), apiClient, "target"); err == nil {
		t.Fatal("expected status error")
	}
}
