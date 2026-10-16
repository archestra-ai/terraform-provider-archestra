package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func testRemoteCard(name string) string {
	value := map[string]any{
		"name": name, "description": "Synthetic external registration.", "version": "1.0.0",
		"supportedInterfaces": []any{map[string]any{"url": "http://127.0.0.1:19998/a2a/" + name, "protocolBinding": "JSONRPC", "protocolVersion": "1.0"}},
		"capabilities":        map[string]any{"streaming": false}, "defaultInputModes": []string{"text/plain"}, "defaultOutputModes": []string{"text/plain"}, "skills": []any{},
		"securitySchemes":      map[string]any{"apiKeyAuth": map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"}, "bearerAuth": map[string]any{"type": "http", "scheme": "bearer"}},
		"securityRequirements": []any{map[string]any{}, map[string]any{"apiKeyAuth": []string{}}, map[string]any{"bearerAuth": []string{}}},
	}
	encoded, err := json.Marshal(map[string]any{"type": "inline_card", "agentCard": value})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
func TestAccRemoteAgentResource(t *testing.T) {
	baseURL := os.Getenv("ARCHESTRA_BASE_URL")
	var metadataPhase, unexpectedAuthentication atomic.Bool
	if os.Getenv("TF_ACC") != "" && baseURL != "" {
		target, err := url.Parse(baseURL)
		if err != nil {
			t.Fatal(err)
		}
		// #nosec G704 -- The acceptance runner explicitly selects this API; the proxy observes requests to that same configured target.
		proxy := httputil.NewSingleHostReverseProxy(target)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if metadataPhase.Load() && r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/a2a/remote-agents/") {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(body, &fields); err != nil {
					t.Error(err)
					return
				}
				if _, ok := fields["auth"]; ok {
					unexpectedAuthentication.Store(true)
				}
				if _, ok := fields["source"]; ok {
					unexpectedAuthentication.Store(true)
				}
			}
			proxy.ServeHTTP(w, r)
		}))
		defer server.Close()
		baseURL = server.URL
	}

	credential := "synthetic-local-only"
	if os.Getenv("ARCHESTRA_READONLY_VAULT_ENABLED") == "true" {
		credential = "secret/data/test/ollama#api_key"
	}
	auth := func(kind string) string {
		v := map[string]any{"type": kind}
		if kind != "none" {
			v["credential"] = credential
		}
		if kind == "api_key" {
			v["headerName"] = "X-API-Key"
		}
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	config := func(name, kind string, enabled bool) string {
		return fmt.Sprintf(`provider "archestra" {base_url=%q}
resource "archestra_remote_agent" "test" {
 source=%q
 auth=%q
 name=%q
 description=""
 enabled=%t
 }`, baseURL, testRemoteCard("tf-remote-card"), auth(kind), name, enabled)
	}
	var saved string
	preserve := func(first bool) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			value := s.RootModule().Resources["archestra_remote_agent.test"].Primary.Attributes["auth"]
			if first {
				saved = value
			} else if value != saved {
				return fmt.Errorf("metadata update changed write-only authentication state")
			}
			if unexpectedAuthentication.Load() {
				return fmt.Errorf("metadata update resent source or authentication configuration")
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-remote-initial", "api_key", false), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_remote_agent.test", "enabled", "false"), resource.TestCheckResourceAttrSet("archestra_remote_agent.test", "connection_id"), preserve(true))},
		{ResourceName: "archestra_remote_agent.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"auth"}},
		{PreConfig: func() { metadataPhase.Store(true) }, Config: config("tf-remote-updated", "api_key", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_remote_agent.test", plancheck.ResourceActionUpdate)}}, Check: preserve(false)},
		{PreConfig: func() { metadataPhase.Store(false) }, Config: config("tf-remote-updated", "api_key", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: config("tf-remote-updated", "bearer", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_remote_agent.test", plancheck.ResourceActionUpdate)}}},
		{Config: config("tf-remote-updated", "none", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_remote_agent.test", plancheck.ResourceActionUpdate)}}},
		{Config: config("tf-remote-updated", "none", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestAccRemoteAgentRejectsProtobufCard(t *testing.T) {
	var source map[string]any
	if err := json.Unmarshal([]byte(testRemoteCard("tf-proto-card")), &source); err != nil {
		t.Fatal(err)
	}
	card, ok := source["agentCard"].(map[string]any)
	if !ok {
		t.Fatal("invalid synthetic Agent Card fixture")
	}
	card["securitySchemes"] = map[string]any{"bearerAuth": map[string]any{"httpAuthSecurityScheme": map[string]any{"scheme": "bearer"}}}
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf(`resource "archestra_remote_agent" "test" {source=%q}`, string(encoded))
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, PlanOnly: true, ExpectError: regexp.MustCompile("Unsupported Agent Card representation")}}})
}
func TestAccAgentRemoteDelegationsResource(t *testing.T) {
	config := func(ids string) string {
		return fmt.Sprintf(`
 resource "archestra_agent" "parent" {name="tf-remote-parent"}
 resource "archestra_remote_agent" "first" {source=%q}
 resource "archestra_remote_agent" "second" {source=%q}
 resource "archestra_agent_remote_delegations" "test" {
 agent_id=archestra_agent.parent.id
 connection_ids=%s
 }`, testRemoteCard("tf-remote-first"), testRemoteCard("tf-remote-second"), ids)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("[archestra_remote_agent.first.connection_id]"), Check: resource.TestCheckResourceAttr("archestra_agent_remote_delegations.test", "connection_ids.#", "1")},
		{ResourceName: "archestra_agent_remote_delegations.test", ImportState: true, ImportStateVerify: true},
		{Config: config("[archestra_remote_agent.first.connection_id,archestra_remote_agent.second.connection_id]"), Check: resource.TestCheckResourceAttr("archestra_agent_remote_delegations.test", "connection_ids.#", "2")},
		{Config: config("[archestra_remote_agent.second.connection_id]"), Check: resource.TestCheckResourceAttr("archestra_agent_remote_delegations.test", "connection_ids.#", "1")},
		{Config: config("[]"), Check: resource.TestCheckResourceAttr("archestra_agent_remote_delegations.test", "connection_ids.#", "0")},
		{Config: config("[]"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
