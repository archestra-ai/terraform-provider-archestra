package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Connector fixtures run only against a numeric loopback origin. Do not permit
// a remote environment or redirects to receive the fixture authorization header.
func knowledgeConnectorFixtureOrigin(t *testing.T) string {
	t.Helper()
	base, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	if base.Scheme != "http" || (base.Hostname() != "localhost" && base.Hostname() != "127.0.0.1" && base.Hostname() != "::1") {
		t.Fatal("knowledge connector fixtures require an isolated loopback HTTP API")
	}
	port := 80
	if base.Port() != "" {
		port, err = strconv.Atoi(base.Port())
		if err != nil || port < 1 || port > 65535 {
			t.Fatal("invalid loopback fixture port")
		}
	}
	return "http://127.0.0.1:" + strconv.Itoa(port)
}

// The fixtures never contact an external service or create credentials.
func testKnowledgeConnectorFixture(t *testing.T, name string) string {
	t.Helper()
	origin := knowledgeConnectorFixtureOrigin(t)
	body, err := json.Marshal(map[string]any{"name": name, "connectorType": "web_crawler", "enabled": false, "config": map[string]any{"type": "web_crawler", "startUrl": "http://127.0.0.1:19999/", "allowPrivateNetwork": true, "maxPages": 1}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/api/connectors", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
	request.Header.Set("Content-Type", "application/json")
	c := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("fixture redirects are refused") }}
	response, err := c.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var connector struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err = json.NewDecoder(response.Body).Decode(&connector); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || connector.ID == "" || connector.Enabled {
		t.Fatalf("disabled connector fixture failed: HTTP %d", response.StatusCode)
	}
	if _, err := uuid.Parse(connector.ID); err != nil {
		t.Fatal("connector API returned an invalid fixture UUID")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodDelete, origin+"/api/connectors/"+connector.ID, nil)
		if err != nil {
			t.Error(err)
			return
		}
		request.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		result, err := c.Do(request)
		if err != nil {
			t.Error(err)
			return
		}
		defer result.Body.Close()
		_, _ = io.Copy(io.Discard, result.Body)
		if result.StatusCode != 200 && result.StatusCode != 204 && result.StatusCode != 404 {
			t.Errorf("owned connector cleanup: HTTP %d", result.StatusCode)
		}
	})
	return connector.ID
}
