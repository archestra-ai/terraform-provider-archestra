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

func githubMetadataGuard(t *testing.T, endpoint, secretField string) (*atomic.Bool, *atomic.Bool) {
	t.Helper()
	metadata := &atomic.Bool{}
	resent := &atomic.Bool{}
	if os.Getenv("TF_ACC") == "" {
		return metadata, resent
	}
	target, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G704 -- The runner explicitly selects the isolated API; this proxy observes requests to the same configured destination.
	proxy := httputil.NewSingleHostReverseProxy(target)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if metadata.Load() && req.Method == http.MethodPut && strings.HasPrefix(req.URL.Path, endpoint) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Error(err)
				http.Error(w, "request inspection failed", 500)
				return
			}
			req.Body = io.NopCloser(bytes.NewReader(body))
			var patch map[string]json.RawMessage
			if err := json.Unmarshal(body, &patch); err != nil {
				t.Error(err)
			}
			if _, present := patch[secretField]; present {
				resent.Store(true)
			}
		}
		proxy.ServeHTTP(w, req)
	}))
	t.Cleanup(server.Close)
	t.Setenv("ARCHESTRA_BASE_URL", server.URL)
	return metadata, resent
}
func TestAccGithubPatResource(t *testing.T) {
	phase, resent := githubMetadataGuard(t, "/api/github-pats/", "token")
	config := func(name, token string) string {
		input := ""
		if token != "" {
			input = fmt.Sprintf("token=%q", token)
		}
		return fmt.Sprintf(`resource "archestra_github_pat" "test" {
 name=%q
 %s
 }`, name, input)
	}
	var id string
	preserve := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_github_pat.test"].Primary.ID != id {
			return fmt.Errorf("stored PAT metadata update replaced its identity")
		}
		if resent.Load() {
			return fmt.Errorf("metadata update resent the write-only PAT")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireByosEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-stored-github-pat", "secret/data/test/ollama#api_key"), Check: func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_github_pat.test"].Primary.ID
			return nil
		}},
		{ResourceName: "archestra_github_pat.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"token"}},
		{PreConfig: func() { phase.Store(true) }, Config: config("tf-renamed-github-pat", "secret/data/test/ollama#api_key"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_github_pat.test", plancheck.ResourceActionUpdate)}}, Check: preserve},
		{Config: config("tf-unmanaged-github-pat", ""), Check: preserve},
		{Config: config("tf-unmanaged-github-pat", ""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{PreConfig: func() { phase.Store(false) }, Config: config("tf-unmanaged-github-pat", "secret/data/test/openai#api_key"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_github_pat.test", plancheck.ResourceActionUpdate)}}, Check: preserve},
	}})
}
func TestAccGithubAppConfigResource(t *testing.T) {
	phase, resent := githubMetadataGuard(t, "/api/github-app-configs/", "privateKey")
	config := func(name, key, installation string) string {
		input := ""
		if key != "" {
			input = fmt.Sprintf("private_key=%q", key)
		}
		return fmt.Sprintf(`resource "archestra_github_app_config" "test" {
 name=%q
 github_url="http://127.0.0.1:19997"
 app_id="100"
 installation_id=%q
 %s
 }`, name, installation, input)
	}
	var id string
	preserve := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_github_app_config.test"].Primary.ID != id {
			return fmt.Errorf("GitHub App metadata update replaced its identity")
		}
		if resent.Load() {
			return fmt.Errorf("metadata update resent the write-only private key")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireByosEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-stored-github-app", "secret/data/test/ollama#api_key", "200"), Check: func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_github_app_config.test"].Primary.ID
			return nil
		}},
		{ResourceName: "archestra_github_app_config.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"private_key"}},
		{PreConfig: func() { phase.Store(true) }, Config: config("tf-renamed-github-app", "secret/data/test/ollama#api_key", "201"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_github_app_config.test", plancheck.ResourceActionUpdate)}}, Check: preserve},
		{Config: config("tf-unmanaged-github-app", "", "201"), Check: preserve},
		{Config: config("tf-unmanaged-github-app", "", "201"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{PreConfig: func() { phase.Store(false) }, Config: config("tf-unmanaged-github-app", "secret/data/test/openai#api_key", "201"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_github_app_config.test", plancheck.ResourceActionUpdate)}}, Check: preserve},
	}})
}
func TestAccGithubRegistrationsRequireCreationCredential(t *testing.T) {
	for _, kind := range []string{"github_pat", "github_app_config"} {
		t.Run(kind, func(t *testing.T) {
			fields := ""
			if kind == "github_app_config" {
				fields = "app_id=\"100\"\ninstallation_id=\"200\""
			}
			resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
				{Config: fmt.Sprintf("resource \"archestra_%s\" \"test\" {\nname=\"tf-missing-input\"\n%s\n}", kind, fields), ExpectError: regexp.MustCompile("Missing credential input")},
			}})
		})
	}
}
