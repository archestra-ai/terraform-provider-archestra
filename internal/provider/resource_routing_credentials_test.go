package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func testCredentialProviderFixture() string {
	return `
 resource "archestra_model_provider" "routing" {
  name="tf-routing-fixture"
  llm_provider="ollama"
  vault_secret_path="secret/data/test/ollama"
  vault_secret_key="api_key"
 }
 resource "archestra_team" "routing" {name="tf-credential-billing"}
 `
}
func testCredentialMetadataStatus(ctx context.Context, endpoint string) (int, error) {
	// #nosec G704 -- The acceptance runner explicitly selects this isolated API; the HTTP client forbids redirects.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, os.Getenv("ARCHESTRA_BASE_URL")+endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	// #nosec G704 -- Acceptance tests explicitly select their isolated API endpoint and forbid redirects.
	response, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = response.Body.Close() }()
	_, err = io.Copy(io.Discard, response.Body)
	return response.StatusCode, err
}
func testOauthCredential(ctx context.Context, id, secret string, valid bool) error {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {id}, "client_secret": {secret}, "scope": {"llm:proxy"}}
	// #nosec G704 -- The acceptance runner explicitly selects this isolated API; the HTTP client forbids redirects.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, os.Getenv("ARCHESTRA_BASE_URL")+"/api/auth/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	// #nosec G704 -- Acceptance tests explicitly select their isolated API endpoint and forbid redirects.
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		return err
	}
	if valid && response.StatusCode != 200 {
		return fmt.Errorf("expected local OAuth issuance, status %d", response.StatusCode)
	}
	if !valid && response.StatusCode != 400 && response.StatusCode != 401 {
		return fmt.Errorf("expected rejected OAuth credential, status %d", response.StatusCode)
	}
	return nil
}
func TestAccVirtualAPIKeyResource(t *testing.T) {
	config := func(name, nonce string, reset bool) string {
		optional := `expires_at="2040-01-01T00:00:00Z"
   billing_team_id=archestra_team.routing.id
   spend_cap={limit_value=100,cleanup_interval="calendar_month"}
   labels=[{key="purpose",value="routing"}]`
		if reset {
			optional = `labels=[]`
		}
		return testCredentialProviderFixture() + fmt.Sprintf(`
 resource "archestra_virtual_key" "test" {
 name=%q
 rotation_nonce=%q
 provider_api_keys=[{provider="ollama",provider_api_key_id=archestra_model_provider.routing.id}]
 %s
 }`, name, nonce, optional)
	}
	var id, secret string
	capture := func(s *terraform.State) error {
		value := s.RootModule().Resources["archestra_virtual_key.test"]
		if value.Primary.Attributes["value"] == "" {
			return fmt.Errorf("creation did not return a sensitive virtual credential")
		}
		id = value.Primary.ID
		secret = value.Primary.Attributes["value"]
		return nil
	}
	preserve := func(s *terraform.State) error {
		value := s.RootModule().Resources["archestra_virtual_key.test"]
		if value.Primary.ID != id || value.Primary.Attributes["value"] != secret {
			return fmt.Errorf("metadata update changed virtual credential identity or value")
		}
		return nil
	}
	rotated := func(s *terraform.State) error {
		value := s.RootModule().Resources["archestra_virtual_key.test"]
		if value.Primary.ID == id || value.Primary.Attributes["value"] == secret {
			return fmt.Errorf("replacement did not rotate virtual credential")
		}
		status, err := testCredentialMetadataStatus(t.Context(), "/api/llm-virtual-keys/"+id)
		if err != nil {
			return err
		}
		if status != 404 {
			return fmt.Errorf("old virtual key was not revoked: status %d", status)
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireByosEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-virtual-router", "1", false), Check: resource.ComposeTestCheckFunc(capture, resource.TestCheckResourceAttr("archestra_virtual_key.test", "spend_cap.limit_value", "100"))},
		// Values are write-once: import adopts metadata and never retrieves the bearer credential.
		{ResourceName: "archestra_virtual_key.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"value", "rotation_nonce"}, ImportStateCheck: func(states []*terraform.InstanceState) error {
			if len(states) != 1 || states[0].Attributes["value"] != "" {
				return fmt.Errorf("import recovered a virtual key value")
			}
			return nil
		}},
		{Config: config("tf-virtual-router-renamed", "1", false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_virtual_key.test", plancheck.ResourceActionUpdate)}}, Check: resource.ComposeTestCheckFunc(preserve, resource.TestCheckResourceAttr("archestra_virtual_key.test", "expires_at", "2040-01-01T00:00:00Z"), resource.TestCheckResourceAttrPair("archestra_virtual_key.test", "billing_team_id", "archestra_team.routing", "id"), resource.TestCheckResourceAttr("archestra_virtual_key.test", "provider_api_keys.#", "1"))},
		{Config: config("tf-virtual-router-renamed", "1", true), Check: resource.ComposeTestCheckFunc(preserve, resource.TestCheckNoResourceAttr("archestra_virtual_key.test", "expires_at"), resource.TestCheckNoResourceAttr("archestra_virtual_key.test", "billing_team_id"), resource.TestCheckNoResourceAttr("archestra_virtual_key.test", "spend_cap.limit_value"), resource.TestCheckResourceAttr("archestra_virtual_key.test", "labels.#", "0"))},
		{Config: config("tf-virtual-router-renamed", "1", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: config("tf-virtual-router-renamed", "2", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_virtual_key.test", plancheck.ResourceActionDestroyBeforeCreate)}}, Check: rotated},
	}})
}
func TestAccVirtualAPIKeyPassthrough(t *testing.T) {
	config := func(name string) string {
		return fmt.Sprintf(`resource "archestra_virtual_key" "test" {
name=%q
 key_type="passthrough"
}`, name)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-passthrough"), Check: resource.TestCheckResourceAttr("archestra_virtual_key.test", "provider_api_keys.#", "0")},
		{ResourceName: "archestra_virtual_key.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"value"}},
		{Config: config("tf-passthrough-renamed"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_virtual_key.test", plancheck.ResourceActionUpdate)}}},
		{Config: config("tf-passthrough-renamed"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestAccVirtualAPIKeyFutureProviders(t *testing.T) {
	config := func(name string) string {
		return fmt.Sprintf(`resource "archestra_virtual_key" "test" {
name=%q
 include_future_providers=true
 provider_api_keys=[]
}`, name)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireEnterprise(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-future-router")},
		{Config: config("tf-future-router-renamed"), Check: resource.TestCheckResourceAttr("archestra_virtual_key.test", "include_future_providers", "true")},
		{Config: config("tf-future-router-renamed"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestAccOauthClientResource(t *testing.T) {
	config := func(name, nonce string, reset bool) string {
		bindings := `service_account_id=archestra_service_account.routing.id
   billing_team_id=archestra_team.routing.id
   spend_cap={limit_value=100,cleanup_interval="calendar_month"}
   labels=[{key="purpose",value="oauth"}]`
		if reset {
			bindings = `labels=[]`
		}
		return testCredentialProviderFixture() + fmt.Sprintf(`
 resource "archestra_service_account" "routing" {
name="tf-oauth-identity"
 role="admin"
}
 resource "archestra_agent" "routing" {name="tf-oauth-agent"}
 resource "archestra_mcp_gateway" "routing" {name="tf-oauth-gateway"}
 resource "archestra_oauth_client" "test" {
 name=%q
 rotation_nonce=%q
 scopes=["mcp","a2a","llm:proxy"]
 allowed_gateway_ids=[archestra_agent.routing.id,archestra_mcp_gateway.routing.id]
 provider_api_keys=[{provider="ollama",provider_api_key_id=archestra_model_provider.routing.id}]
 %s
 }`, name, nonce, bindings)
	}
	var id, clientID, secret string
	capture := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_oauth_client.test"]
		id = v.Primary.ID
		clientID = v.Primary.Attributes["client_id"]
		secret = v.Primary.Attributes["client_secret"]
		if secret == "" {
			return fmt.Errorf("creation did not return an OAuth secret")
		}
		if err := testOauthCredential(t.Context(), clientID, "deliberately-invalid-secret", false); err != nil {
			return err
		}
		return testOauthCredential(t.Context(), clientID, secret, true)
	}
	preserve := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_oauth_client.test"]
		if v.Primary.ID != id || v.Primary.Attributes["client_id"] != clientID || v.Primary.Attributes["client_secret"] != secret {
			return fmt.Errorf("metadata update changed OAuth identity or secret")
		}
		return nil
	}
	rotated := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_oauth_client.test"]
		if v.Primary.ID != id || v.Primary.Attributes["client_id"] != clientID || v.Primary.Attributes["client_secret"] == secret {
			return fmt.Errorf("OAuth rotation failed to preserve identity and replace the secret")
		}
		if err := testOauthCredential(t.Context(), clientID, secret, false); err != nil {
			return err
		}
		return testOauthCredential(t.Context(), clientID, v.Primary.Attributes["client_secret"], true)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireByosEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-unified-oauth", "1", false), Check: capture},
		{ResourceName: "archestra_oauth_client.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"client_secret", "rotation_nonce"}},
		{Config: config("tf-unified-oauth-renamed", "1", false), Check: resource.ComposeTestCheckFunc(preserve, resource.TestCheckResourceAttrPair("archestra_oauth_client.test", "service_account_id", "archestra_service_account.routing", "id"), resource.TestCheckResourceAttrPair("archestra_oauth_client.test", "billing_team_id", "archestra_team.routing", "id"), resource.TestCheckResourceAttr("archestra_oauth_client.test", "spend_cap.limit_value", "100"))},
		{Config: config("tf-unified-oauth-renamed", "2", false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_oauth_client.test", plancheck.ResourceActionUpdate)}}, Check: rotated},
		{Config: config("tf-unified-oauth-renamed", "2", true), Check: resource.ComposeTestCheckFunc(resource.TestCheckNoResourceAttr("archestra_oauth_client.test", "service_account_id"), resource.TestCheckNoResourceAttr("archestra_oauth_client.test", "billing_team_id"), resource.TestCheckNoResourceAttr("archestra_oauth_client.test", "spend_cap.limit_value"), resource.TestCheckResourceAttr("archestra_oauth_client.test", "labels.#", "0"))},
		{Config: config("tf-unified-oauth-renamed", "2", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestAccOauthClientAuthorizationCode(t *testing.T) {
	config := func(name, redirect string) string {
		return fmt.Sprintf(`resource "archestra_oauth_client" "test" {
name=%q
 grant_type="authorization_code"
 scopes=["mcp","a2a","llm:proxy"]
 redirect_uris=[%q]
}`, name, redirect)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-user-oauth", "https://example.test/callback")},
		{ResourceName: "archestra_oauth_client.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"client_secret"}},
		{Config: config("tf-user-oauth-renamed", "https://example.test/callback"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_oauth_client.test", plancheck.ResourceActionUpdate)}}},
		{Config: config("tf-user-oauth-renamed", "https://example.test/updated")},
		{Config: config("tf-user-oauth-renamed", "https://example.test/updated"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestAccOauthClientFutureProviders(t *testing.T) {
	config := func(name string) string {
		return fmt.Sprintf(`resource "archestra_oauth_client" "test" {
name=%q
 scopes=["llm:proxy"]
 include_future_providers=true
 provider_api_keys=[]
}`, name)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireEnterprise(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-future-oauth")},
		{Config: config("tf-future-oauth-renamed"), Check: resource.TestCheckResourceAttr("archestra_oauth_client.test", "include_future_providers", "true")},
		{Config: config("tf-future-oauth-renamed"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
