package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	connectorTestID = "44444444-4444-4444-8444-444444444444"
	connectorKBOne  = "55555555-5555-4555-8555-555555555555"
	connectorKBTwo  = "66666666-6666-4666-8666-666666666666"
)

// fakeConnectorAPI emulates the connector endpoints the resource uses,
// including the server-side config normalization and defaults.
type fakeConnectorAPI struct {
	t              *testing.T
	mu             sync.Mutex
	connector      map[string]any
	knowledgeBases map[string]bool
	writes         []map[string]any
}

func (f *fakeConnectorAPI) serve(w http.ResponseWriter, req *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	path := req.URL.Path
	switch {
	case path == "/health":
		fmt.Fprint(w, `{"version":"1.5.0-rc.32"}`)
	case !strings.HasPrefix(path, "/api/connectors"):
		// Permission policies: unreadable, so unmanaged sharing is skipped.
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"message":"forbidden","type":"api_forbidden"}}`)
	case req.Method == http.MethodPost && path == "/api/connectors":
		body := f.decode(req)
		f.connector = map[string]any{
			"id": connectorTestID, "organizationId": "org", "name": body["name"], "description": body["description"],
			"connectorType": body["connectorType"], "config": normalizeFakeConfig(body["config"]),
			"schedule": valueOr(body["schedule"], "0 */6 * * *"), "ftsLanguage": valueOr(body["ftsLanguage"], "english"),
			"permissionSyncIntervalSeconds": valueOr(body["permissionSyncIntervalSeconds"], float64(3600)),
			"enabled":                       valueOr(body["enabled"], true), "syncPermissionsFromSource": valueOr(body["syncPermissionsFromSource"], false),
			"environmentId": "77777777-7777-4777-8777-777777777777", "labels": []any{}, "createdAt": "2026-11-13T00:00:00Z",
			"updatedAt": "2026-11-13T00:00:00Z", "deletedAt": nil, "visibility": "org-wide", "teamIds": []any{},
		}
		f.knowledgeBases = map[string]bool{}
		if ids, ok := body["knowledgeBaseIds"].([]any); ok {
			for _, id := range ids {
				f.knowledgeBases[mustAs[string](id)] = true
			}
		}
		f.write(w, f.connector)
	case f.connector == nil:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"message":"Connector not found","type":"api_not_found"}}`)
	case path == "/api/connectors/"+connectorTestID:
		switch req.Method {
		case http.MethodGet:
			f.write(w, f.connector)
		case http.MethodPut:
			body := f.decode(req)
			for key, value := range body {
				switch key {
				case "config":
					f.connector["config"] = normalizeFakeConfig(value)
				case "visibility":
					f.connector["syncPermissionsFromSource"] = value == "auto-sync-permissions"
				case "credentials":
				default:
					f.connector[key] = value
				}
			}
			f.write(w, f.connector)
		case http.MethodDelete:
			f.connector = nil
			fmt.Fprint(w, `{"success":true}`)
		}
	case path == "/api/connectors/"+connectorTestID+"/knowledge-bases":
		if req.Method == http.MethodPost {
			for _, id := range mustAs[[]any](f.decode(req)["knowledgeBaseIds"]) {
				f.knowledgeBases[mustAs[string](id)] = true
			}
			fmt.Fprint(w, `{"success":true}`)
			return
		}
		data := []map[string]string{}
		for id := range f.knowledgeBases {
			data = append(data, map[string]string{"id": id})
		}
		sort.Slice(data, func(i, j int) bool { return data[i]["id"] < data[j]["id"] })
		f.write(w, map[string]any{"data": data})
	case req.Method == http.MethodDelete && strings.HasPrefix(path, "/api/connectors/"+connectorTestID+"/knowledge-bases/"):
		delete(f.knowledgeBases, strings.TrimPrefix(path, "/api/connectors/"+connectorTestID+"/knowledge-bases/"))
		fmt.Fprint(w, `{"success":true}`)
	default:
		f.t.Errorf("unexpected request: %s %s", req.Method, path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeConnectorAPI) decode(req *http.Request) map[string]any {
	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		f.t.Errorf("decode %s %s: %v", req.Method, req.URL.Path, err)
	}
	f.writes = append(f.writes, map[string]any{"method": req.Method, "body": body})
	return body
}

func (f *fakeConnectorAPI) write(w http.ResponseWriter, value any) {
	if err := json.NewEncoder(w).Encode(value); err != nil {
		f.t.Error(err)
	}
}

func (f *fakeConnectorAPI) lastWrite(t *testing.T, method string) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.writes) - 1; i >= 0; i-- {
		if f.writes[i]["method"] == method {
			return mustAs[map[string]any](f.writes[i]["body"])
		}
	}
	t.Fatalf("no %s request recorded", method)
	return nil
}

// normalizeFakeConfig adds a protocol and drops trailing slashes on startUrl
// and adds a server default, as the API does.
func normalizeFakeConfig(value any) map[string]any {
	config, _ := value.(map[string]any)
	out := map[string]any{"maxDepth": float64(3)}
	for k, v := range config {
		out[k] = v
	}
	if url, ok := out["startUrl"].(string); ok {
		if !strings.Contains(url, "://") {
			url = "https://" + url
		}
		out["startUrl"] = strings.TrimRight(url, "/")
	}
	return out
}

func valueOr(value, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}

func connectorTestConfig(serverURL, body string) string {
	return fmt.Sprintf(`provider "archestra" {
  base_url = %q
  api_key  = "fixture-key"
}
%s`, serverURL, body)
}

// Exercises plan, apply, refresh and import against a controlled HTTP
// boundary: config normalization must not produce drift, permission sync is
// switched through visibility, and assignments use their own endpoints.
func TestKnowledgeConnectorTerraformFramework(t *testing.T) {
	fake := &fakeConnectorAPI{t: t}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	defer server.Close()

	create := connectorTestConfig(server.URL, fmt.Sprintf(`resource "archestra_knowledge_connector" "test" {
  name               = "Docs site"
  connector_type     = "web_crawler"
  config             = jsonencode({ startUrl = "docs.example.com/", maxPages = 5 })
  knowledge_base_ids = [%q]
}`, connectorKBOne))
	update := connectorTestConfig(server.URL, fmt.Sprintf(`resource "archestra_knowledge_connector" "test" {
  name                         = "Docs site"
  connector_type               = "web_crawler"
  config                       = jsonencode({ startUrl = "docs.example.com/", maxPages = 10 })
  schedule                     = "0 3 * * *"
  sync_permissions_from_source = true
  knowledge_base_ids           = [%q]
}`, connectorKBTwo))

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: create,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "id", connectorTestID),
					tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "schedule", "0 */6 * * *"),
					tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "enabled", "true"),
					tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "sync_permissions_from_source", "false"),
					tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "knowledge_base_ids.#", "1"),
					func(_ *terraform.State) error {
						body := fake.lastWrite(t, http.MethodPost)
						config := mustAs[map[string]any](body["config"])
						if config["type"] != "web_crawler" {
							return fmt.Errorf("create config.type = %v, want web_crawler", config["type"])
						}
						if _, ok := body["credentials"]; ok {
							return fmt.Errorf("create sent credentials for a web crawler")
						}
						return nil
					},
				),
			},
			{
				Config: update,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "schedule", "0 3 * * *"),
					tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "sync_permissions_from_source", "true"),
					tfresource.TestCheckTypeSetElemAttr("archestra_knowledge_connector.test", "knowledge_base_ids.*", connectorKBTwo),
					tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "knowledge_base_ids.#", "1"),
					func(_ *terraform.State) error {
						body := fake.lastWrite(t, http.MethodPut)
						if body["visibility"] != "auto-sync-permissions" {
							return fmt.Errorf("update visibility = %v, want auto-sync-permissions", body["visibility"])
						}
						for _, key := range []string{"syncPermissionsFromSource", "knowledgeBaseIds", "connectorType", "credentials"} {
							if _, ok := body[key]; ok {
								return fmt.Errorf("update sent %s", key)
							}
						}
						if mustAs[map[string]any](body["config"])["type"] != "web_crawler" {
							return fmt.Errorf("update config lost its type")
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "archestra_knowledge_connector.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config", "knowledge_base_ids", "permissions"},
			},
		},
	})
}

// Credentials are write-only: they are sent at create and again only when
// changed, and state keeps the configured values.
func TestKnowledgeConnectorCredentialsWriteOnly(t *testing.T) {
	fake := &fakeConnectorAPI{t: t}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	defer server.Close()
	config := func(name, token string) string {
		return connectorTestConfig(server.URL, fmt.Sprintf(`resource "archestra_knowledge_connector" "test" {
  name           = %q
  connector_type = "jira"
  config         = jsonencode({ jiraBaseUrl = "https://example.atlassian.net", isCloud = true })
  credentials    = { email = "bot@example.com", api_token = %q }
}`, name, token))
	}
	credentialsSent := func(method string, want bool) tfresource.TestCheckFunc {
		return func(_ *terraform.State) error {
			body := fake.lastWrite(t, method)
			credentials, ok := body["credentials"].(map[string]any)
			if ok != want {
				return fmt.Errorf("%s credentials sent = %t, want %t", method, ok, want)
			}
			if want && credentials["apiToken"] == nil {
				return fmt.Errorf("%s credentials lack apiToken: %v", method, credentials)
			}
			return nil
		}
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{Config: config("Jira", "token-one"), Check: tfresource.ComposeTestCheckFunc(
				credentialsSent(http.MethodPost, true),
				tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "credentials.api_token", "token-one"),
			)},
			{Config: config("Jira renamed", "token-one"), Check: credentialsSent(http.MethodPut, false)},
			{Config: config("Jira renamed", "token-two"), Check: tfresource.ComposeTestCheckFunc(
				credentialsSent(http.MethodPut, true),
				tfresource.TestCheckResourceAttr("archestra_knowledge_connector.test", "credentials.api_token", "token-two"),
			)},
		},
	})
}

func TestKnowledgeConnectorValidation(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"web crawler credentials": {
			body: `connector_type = "web_crawler"
  config = jsonencode({ startUrl = "https://example.com" })
  credentials = { api_token = "x" }`,
			want: "Web crawler connectors take no credentials",
		},
		"conflicting type": {
			body: `connector_type = "jira"
  config = jsonencode({ type = "github" })`,
			want: "config.type is \"github\"",
		},
		"non-object config": {
			body: `connector_type = "jira"
  config = jsonencode(["a"])`,
			want: "config must be a JSON object",
		},
		"unknown type": {
			body: `connector_type = "file_upload"
  config = jsonencode({})`,
			want: "value must be one of",
		},
	} {
		t.Run(name, func(t *testing.T) {
			tfresource.UnitTest(t, tfresource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []tfresource.TestStep{{
					Config: connectorTestConfig("http://127.0.0.1:1", `resource "archestra_knowledge_connector" "test" {
  name = "x"
  `+tc.body+`
}`),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(regexp.QuoteMeta(tc.want)),
				}},
			})
		})
	}
}

func TestKnowledgeConnectorConfigFromAPI(t *testing.T) {
	prior := jsontypes.NewNormalizedValue(`{"startUrl":"docs.example.com/","maxPages":5}`)
	for name, tc := range map[string]struct {
		server   string
		keepsOld bool
	}{
		"normalized url and server default": {server: `{"type":"web_crawler","startUrl":"https://docs.example.com","maxPages":5,"maxDepth":3}`, keepsOld: true},
		"changed configured key":            {server: `{"type":"web_crawler","startUrl":"https://docs.example.com","maxPages":9}`},
		"removed configured key":            {server: `{"type":"web_crawler","startUrl":"https://docs.example.com"}`},
		"different host":                    {server: `{"type":"web_crawler","startUrl":"https://other.example.com","maxPages":5}`},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := knowledgeConnectorConfigFromAPI(prior, json.RawMessage(tc.server))
			if err != nil {
				t.Fatal(err)
			}
			if tc.keepsOld != got.Equal(prior) {
				t.Fatalf("kept configured value = %t, want %t (got %s)", got.Equal(prior), tc.keepsOld, got.ValueString())
			}
			if strings.Contains(got.ValueString(), `"type"`) {
				t.Fatalf("type discriminator leaked into state: %s", got.ValueString())
			}
		})
	}
	imported, err := knowledgeConnectorConfigFromAPI(jsontypes.NewNormalizedNull(), json.RawMessage(`{"type":"jira","isCloud":true}`))
	if err != nil || imported.ValueString() != `{"isCloud":true}` {
		t.Fatalf("import config = %q, %v", imported.ValueString(), err)
	}
}
