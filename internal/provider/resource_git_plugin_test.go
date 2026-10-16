package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const gitPluginFixtureSHA = "9e6a43f5347f6f3730ffeb75946f6e162791851d"
const gitPluginFixtureSubdir = "scripts/integration/fixtures/plugins/synthetic"

func gitPluginFixtureConfig(name, description string, enabled bool, sourceControls string, excluded bool) string {
	exclude := `[]`
	if excluded {
		exclude = `["excluded.txt"]`
	}
	return fmt.Sprintf(`resource "archestra_github_pat" "fixture" {
 name="tf-plugin-auth-fixture"
 token="secret/data/test/ollama#api_key"
 }
 resource "archestra_git_plugin" "test" {
 display_name=%q
 description=%q
 enabled=%t
 client_type="claude-code"
 supported_platforms=["posix"]
 source_repo="archestra-ai/terraform-provider-archestra"
 source_sha=%q
 source_subdir=%q
 source_exclude=%s
 %s
 }`, name, description, enabled, gitPluginFixtureSHA, gitPluginFixtureSubdir, exclude, sourceControls)
}
func TestAccGitPluginResource(t *testing.T) {
	phase, resent := metadataWriteGuard(t, "/api/plugins/", "files")
	sourcePhase, sourceResent := metadataWriteGuard(t, "/api/plugins/", "githubSource")
	var id, hash string
	capture := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_git_plugin.test"]
		id = v.Primary.ID
		hash = v.Primary.Attributes["content_hash"]
		if hash == "" {
			return fmt.Errorf("missing approved content hash")
		}
		return nil
	}
	preserved := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_git_plugin.test"]
		if v.Primary.ID != id || v.Primary.Attributes["content_hash"] != hash || resent.Load() || sourceResent.Load() {
			return fmt.Errorf("metadata resent source controls or changed approved content")
		}
		return nil
	}
	replaced := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_git_plugin.test"]
		if v.Primary.ID == id || v.Primary.Attributes["content_hash"] == hash {
			return fmt.Errorf("changed source selection did not replace approved package")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireByosEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: gitPluginFixtureConfig("tf-pinned-plugin", "Initial", false, `labels=[{key="purpose",value="source"}]`, true), Check: resource.ComposeTestCheckFunc(capture, resource.TestCheckResourceAttr("archestra_git_plugin.test", "enabled", "false"), resource.TestCheckResourceAttr("archestra_git_plugin.test", "source_kind", "github"), resource.TestCheckResourceAttr("archestra_git_plugin.test", "files.%", "2"), resource.TestCheckResourceAttr("archestra_git_plugin.test", "source_sha", gitPluginFixtureSHA))},
		{ResourceName: "archestra_git_plugin.test", ImportState: true, ImportStateVerify: true},
		{PreConfig: func() { phase.Store(true); sourcePhase.Store(true) }, Config: gitPluginFixtureConfig("tf-renamed-source-plugin", "Metadata only", true, `labels=[{key="purpose",value="source"}]`, true), Check: preserved},
		{PreConfig: func() { phase.Store(false); sourcePhase.Store(false) }, Config: gitPluginFixtureConfig("tf-renamed-source-plugin", "Metadata only", true, `labels=[]
 github_sync_ref="`+gitPluginFixtureSHA+`"
 github_sync_interval="1d"`, true), Check: resource.ComposeTestCheckFunc(preserved, resource.TestCheckResourceAttr("archestra_git_plugin.test", "github_sync_interval", "1d"))},
		{Config: gitPluginFixtureConfig("tf-renamed-source-plugin", "Metadata only", true, `labels=[]
 github_pat_id=archestra_github_pat.fixture.id`, true), Check: resource.ComposeTestCheckFunc(preserved, resource.TestCheckResourceAttrPair("archestra_git_plugin.test", "github_pat_id", "archestra_github_pat.fixture", "id"), resource.TestCheckNoResourceAttr("archestra_git_plugin.test", "github_sync_interval"))},
		{Config: gitPluginFixtureConfig("tf-renamed-source-plugin", "Metadata only", true, `labels=[]`, true), Check: resource.ComposeTestCheckFunc(preserved, resource.TestCheckNoResourceAttr("archestra_git_plugin.test", "github_pat_id"), resource.TestCheckNoResourceAttr("archestra_git_plugin.test", "github_sync_ref"), resource.TestCheckResourceAttr("archestra_git_plugin.test", "labels.#", "0"))},
		{Config: gitPluginFixtureConfig("tf-renamed-source-plugin", "Metadata only", true, `labels=[]`, true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: gitPluginFixtureConfig("tf-renamed-source-plugin", "Metadata only", true, `labels=[]`, false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_git_plugin.test", plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.ComposeTestCheckFunc(replaced, resource.TestCheckResourceAttr("archestra_git_plugin.test", "files.%", "3"))},
	}})
}
func TestAccGitPluginRecoversFollowupFailure(t *testing.T) {
	var failed atomic.Bool
	var imports atomic.Int32
	var firstID atomic.Pointer[string]
	if os.Getenv("TF_ACC") != "" {
		target, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
		if err != nil {
			t.Fatal(err)
		}
		// #nosec G704 -- The runner explicitly selects the isolated API; this proxy injects one failed follow-up to the same destination.
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(response *http.Response) error {
			if response.Request.Method == http.MethodPost && response.Request.URL.Path == "/api/plugins/github/import" && response.StatusCode == 200 {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					return err
				}
				_ = response.Body.Close()
				response.Body = io.NopCloser(bytes.NewReader(body))
				var value struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(body, &value); err != nil {
					return err
				}
				if firstID.Load() == nil {
					firstID.Store(&value.ID)
				}
			}
			return nil
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodPost && req.URL.Path == "/api/plugins/github/import" {
				imports.Add(1)
			}
			if req.Method == http.MethodPut && strings.HasPrefix(req.URL.Path, "/api/plugins/") && failed.CompareAndSwap(false, true) {
				http.Error(w, "synthetic follow-up failure", http.StatusServiceUnavailable)
				return
			}
			proxy.ServeHTTP(w, req)
		}))
		t.Cleanup(server.Close)
		t.Setenv("ARCHESTRA_BASE_URL", server.URL)
	}
	config := gitPluginFixtureConfig("tf-retry-source-plugin", "", false, `github_sync_ref="`+gitPluginFixtureSHA+`"`, true)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireByosEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, ExpectError: regexp.MustCompile("Update Failed")},
		{Config: config, Check: func(s *terraform.State) error {
			v := s.RootModule().Resources["archestra_git_plugin.test"].Primary
			first := firstID.Load()
			if imports.Load() != 2 || first == nil || v.ID == *first || v.Attributes["enabled"] != "false" {
				return fmt.Errorf("tainted-create recovery did not replace the partial identity and apply its control")
			}
			// #nosec G704 -- The acceptance runner explicitly selects this API; only the captured owned plugin UUID is appended.
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, os.Getenv("ARCHESTRA_BASE_URL")+"/api/plugins/"+*first, nil)
			if err != nil {
				return err
			}
			request.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			// #nosec G704 -- This request goes to the same explicitly configured isolated acceptance API.
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				return err
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != 404 {
				return fmt.Errorf("failed-create identity was not normally deleted: status%d", response.StatusCode)
			}
			return nil
		}},
	}})
}
func TestAccGitPluginRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct{ name, config, match string }{
		{"commit", strings.Replace(gitPluginFixtureConfig("invalid-source", "", false, "", true), gitPluginFixtureSHA, "main", 1), "approved lowercase"},
		{"subdir", strings.Replace(gitPluginFixtureConfig("invalid-source", "", false, "", true), gitPluginFixtureSubdir, "../outside", 1), "Invalid source subtree"},
		{"repo", strings.Replace(gitPluginFixtureConfig("invalid-source", "", false, "", true), `source_repo="archestra-ai/terraform-provider-archestra"`, `source_repo="archestra-ai/terraform-provider-archestra.git"`, 1), "Noncanonical repository"},
		{"credentials", gitPluginFixtureConfig("invalid-source", "", false, `github_pat_id="11111111-1111-4111-8111-111111111111"
 github_app_config_id="22222222-2222-4222-8222-222222222222"`, true), "Invalid Attribute Combination"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: tc.config, ExpectError: regexp.MustCompile(tc.match)}}})
		})
	}
}
func TestPluginSourceAdoptionRejectsOtherKind(t *testing.T) {
	for _, git := range []bool{false, true} {
		r := &PluginResource{gitSource: git}
		body := `{"sourceKind":"github"}`
		if git {
			body = `{"sourceKind":"manual"}`
		}
		data := PluginResourceModel{ID: types.StringValue("retained-id")}
		var d diag.Diagnostics
		r.flatten(context.Background(), &data, []byte(body), &d)
		if !d.HasError() || data.ID.ValueString() != "retained-id" {
			t.Fatalf("source mismatch changed state: %v", d)
		}
	}
}
