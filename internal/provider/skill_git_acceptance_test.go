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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Published commits keep these read-only source fixtures reproducible.
const skillGitFixtureSHA = "411935fb1648ca812dff1f3503c0be77db65bcf3"
const skillGitAlternateSHA = "453ee6fb4cb0ad3564f588114dbdaf891a10a488"

func skillGitFixtureConfig(commit, interval, settings string) string {
	return fmt.Sprintf(`resource "archestra_skill" "test" {
 git_source={
 repo_url="https://github.com/archestra-ai/terraform-provider-archestra/tree/%s"
 skill_path="scripts/integration/fixtures/skills/synthetic"
 sync_interval=%q
 }
 environment_ids=[]
 permissions=[]
 labels=[]
 %s
 }`, commit, interval, settings)
}

func TestAccGitSkillResource(t *testing.T) {
	phase, resent := metadataWriteGuard(t, "/api/skills/", "files")
	var id string
	capture := func(s *terraform.State) error {
		id = s.RootModule().Resources["archestra_skill.test"].Primary.ID
		return nil
	}
	preserved := func(s *terraform.State) error {
		row := s.RootModule().Resources["archestra_skill.test"].Primary
		if row.ID != id || resent.Load() {
			return fmt.Errorf("Git Skill settings changed identity or resent files")
		}
		return nil
	}
	first := skillGitFixtureConfig(skillGitFixtureSHA, "1d", `icon="book"`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: first, Check: resource.ComposeTestCheckFunc(capture, resource.TestCheckResourceAttr("archestra_skill.test", "name", "provider-synthetic-import"), resource.TestCheckResourceAttr("archestra_skill.test", "source_type", "github"), resource.TestCheckResourceAttr("archestra_skill.test", "source_commit", skillGitFixtureSHA), resource.TestCheckResourceAttr("archestra_skill.test", "latest_version", "1"))},
		// A background sync can finish between apply and import and set last_synced_at.
		{ResourceName: "archestra_skill.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"last_synced_at"}},
		{PreConfig: func() { phase.Store(true) }, Config: skillGitFixtureConfig(skillGitFixtureSHA, "1d", `icon="books"`), Check: resource.ComposeTestCheckFunc(preserved, resource.TestCheckResourceAttr("archestra_skill.test", "latest_version", "1"))},
		{Config: skillGitFixtureConfig(skillGitFixtureSHA, "1h", `icon="books"`), Check: resource.ComposeTestCheckFunc(preserved, resource.TestCheckResourceAttr("archestra_skill.test", "git_source.sync_interval", "1h"), resource.TestCheckResourceAttr("archestra_skill.test", "latest_version", "1"))},
		{Config: skillGitFixtureConfig(skillGitAlternateSHA, "1h", `icon="books"`), Check: resource.ComposeTestCheckFunc(preserved, resource.TestCheckResourceAttr("archestra_skill.test", "source_commit", skillGitAlternateSHA))},
		{Config: skillGitFixtureConfig(skillGitAlternateSHA, "1h", `icon="books"`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccGitSkillRecoversFollowupFailure(t *testing.T) {
	var failed atomic.Bool
	var imports atomic.Int32
	var firstID atomic.Pointer[string]
	if os.Getenv("TF_ACC") != "" {
		target, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
		if err != nil {
			t.Fatal(err)
		}
		// #nosec G704 -- The acceptance runner selects this isolated API. This proxy injects one owned Skill follow-up failure.
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(response *http.Response) error {
			if response.Request.Method == http.MethodPost && response.Request.URL.Path == "/api/skills/github/import" && response.StatusCode == 200 {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					return err
				}
				_ = response.Body.Close()
				response.Body = io.NopCloser(bytes.NewReader(body))
				var value struct {
					Created []struct {
						ID string `json:"id"`
					} `json:"created"`
				}
				if err := json.Unmarshal(body, &value); err != nil {
					return err
				}
				if firstID.Load() == nil && len(value.Created) == 1 {
					id := value.Created[0].ID
					firstID.Store(&id)
				}
			}
			return nil
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodPost && req.URL.Path == "/api/skills/github/import" {
				imports.Add(1)
			}
			if req.Method == http.MethodPut && strings.HasPrefix(req.URL.Path, "/api/skills/") && failed.CompareAndSwap(false, true) {
				http.Error(w, "synthetic follow-up failure", http.StatusServiceUnavailable)
				return
			}
			proxy.ServeHTTP(w, req)
		}))
		t.Cleanup(server.Close)
		t.Setenv("ARCHESTRA_BASE_URL", server.URL)
	}
	config := skillGitFixtureConfig(skillGitFixtureSHA, "1d", `icon="book"`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, ExpectError: regexp.MustCompile("Git Skill settings update failed")},
		{Config: config, Check: func(s *terraform.State) error {
			first := firstID.Load()
			next := s.RootModule().Resources["archestra_skill.test"].Primary
			if imports.Load() != 2 || first == nil || next.ID == *first || next.Attributes["icon"] != "book" {
				return fmt.Errorf("failed-create recovery did not replace the partial identity and apply settings")
			}
			// #nosec G704 -- This checks only the captured, owned Skill UUID at the runner's explicitly configured isolated API.
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, os.Getenv("ARCHESTRA_BASE_URL")+"/api/skills/"+*first, nil)
			if err != nil {
				return err
			}
			request.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			// #nosec G704 -- Same authorized isolated API; no destination comes from response data.
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

func TestAccSkillSourceKindReplacement(t *testing.T) {
	var id string
	capture := func(s *terraform.State) error {
		id = s.RootModule().Resources["archestra_skill.test"].Primary.ID
		return nil
	}
	changed := func(s *terraform.State) error {
		next := s.RootModule().Resources["archestra_skill.test"].Primary.ID
		if next == id {
			return fmt.Errorf("source-kind change did not replace Skill")
		}
		id = next
		return nil
	}
	manual := `resource "archestra_skill" "test" {
 name="tf-skill-source-mode"
 description="Synthetic authored content"
 content="Inspect the synthetic fixture only."
 files={}
 labels=[]
 environment_ids=[]
 permissions=[]
 }`
	git := skillGitFixtureConfig(skillGitFixtureSHA, "1d", "")
	replacement := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_skill.test", plancheck.ResourceActionDestroyBeforeCreate)}}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: manual, Check: capture},
		{Config: git, ConfigPlanChecks: replacement, Check: changed},
		{Config: manual, ConfigPlanChecks: replacement, Check: changed},
		{Config: manual, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccGitSkillRejectsMixedContent(t *testing.T) {
	config := skillGitFixtureConfig(skillGitFixtureSHA, "1d", `content="Conflicting authored content."`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile("Git Skill content is repository-owned")}}})
}
