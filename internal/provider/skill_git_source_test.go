package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const skillGitTestID = "11111111-1111-4111-8111-111111111111"
const skillGitTestRepo = "archestra-ai/terraform-provider-archestra"
const skillGitTestPath = "scripts/integration/fixtures/skills/synthetic"

func skillGitTestSource(t *testing.T, edit func(*skillGitSourceModel)) types.Object {
	t.Helper()
	s := skillGitSourceModel{RepoURL: types.StringValue("https://github.com/" + skillGitTestRepo + "/tree/main"), SkillPath: types.StringValue(skillGitTestPath), Provider: types.StringValue("github"), SyncInterval: types.StringValue("1d"), GitHubAppID: types.StringNull(), GitHubPATID: types.StringNull(), GitLabTokenID: types.StringNull()}
	if edit != nil {
		edit(&s)
	}
	v, d := types.ObjectValueFrom(t.Context(), skillGitSourceType.AttrTypes, s)
	if d.HasError() {
		t.Fatal(d)
	}
	return v
}

func skillGitTestDetail(interval string) string {
	return fmt.Sprintf(`{"id":%q,"name":"git-fixture","description":"Git fixture","content":"Repository owned.","templated":true,"metadata":{"z":"last","a":"first"},"allowedTools":"Write Read","latestVersion":1,"files":[{"path":"references/a.txt","content":"file","encoding":"utf8"}],"sourceType":"github","sourceRef":%q,"sourceCommit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","githubSyncRef":"main","githubSyncInterval":%s,"sourceProvider":"github","labels":[],"environments":[]}`, skillGitTestID, skillGitTestRepo+"@main:"+skillGitTestPath, interval)
}

func skillGitTestObserved(t *testing.T) SkillResourceModel {
	t.Helper()
	m := skillTestModel()
	m.GitSource = skillGitTestSource(t, nil)
	var d diag.Diagnostics
	m.populate(t.Context(), []byte(skillGitTestDetail(`"1d"`)), &d)
	if d.HasError() {
		t.Fatal(d)
	}
	return m
}

func skillGitTestResource(t *testing.T, handler http.HandlerFunc) *SkillResource {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &SkillResource{client: c}
}

func skillGitTestPlanState(t *testing.T, r *SkillResource, before, after SkillResourceModel) (tfsdk.State, tfsdk.Plan) {
	t.Helper()
	var s resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &s)
	state := tfsdk.State{Schema: s.Schema}
	plan := tfsdk.Plan{Schema: s.Schema}
	if d := state.Set(t.Context(), before); d.HasError() {
		t.Fatal(d)
	}
	if d := plan.Set(t.Context(), after); d.HasError() {
		t.Fatal(d)
	}
	return state, plan
}

func TestSkillGitSettingsEchoPreservesSourceManifest(t *testing.T) {
	var d diag.Diagnostics
	encoded := skillGitSettingsBody([]byte(skillGitTestDetail(`"1d"`)), map[string]any{"icon": "book", "files": []any{}}, &d)
	if d.HasError() {
		t.Fatal(d)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["files"]; ok {
		t.Fatal("settings update echoed repository-owned files")
	}
	var content string
	if err := json.Unmarshal(body["content"], &content); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `"metadata":{"z":"last","a":"first"}`) || !strings.Contains(content, `"allowed-tools":"Write Read"`) || !strings.HasSuffix(content, "Repository owned.") || string(body["baseVersion"]) != "1" {
		t.Fatalf("content or revision changed: %s", encoded)
	}
}

func TestSkillGitSettingsUpdateDoesNotPullOrRewriteFiles(t *testing.T) {
	puts, patches := 0, 0
	r := skillGitTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case http.MethodGet:
			fmt.Fprint(w, skillGitTestDetail(`"1d"`))
		case http.MethodPatch:
			patches++
			t.Error("settings update pulled the repository")
			w.WriteHeader(500)
		case http.MethodPut:
			puts++
			var body map[string]json.RawMessage
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if _, ok := body["files"]; ok {
				t.Error("settings update sent files")
			}
			var content string
			_ = json.Unmarshal(body["content"], &content)
			if !strings.Contains(content, `"metadata":{"z":"last","a":"first"}`) {
				t.Error("metadata key order changed")
			}
			var detail map[string]any
			_ = json.Unmarshal([]byte(skillGitTestDetail(`"1d"`)), &detail)
			detail["icon"] = "book"
			_ = json.NewEncoder(w).Encode(detail)
		default:
			t.Errorf("unexpected method %s", req.Method)
			w.WriteHeader(500)
		}
	})
	before := skillGitTestObserved(t)
	after := before
	after.Icon = types.StringValue("book")
	state, plan := skillGitTestPlanState(t, r, before, after)
	response := resource.UpdateResponse{State: state}
	r.Update(t.Context(), resource.UpdateRequest{Plan: plan, State: state}, &response)
	if response.Diagnostics.HasError() || puts != 1 || patches != 0 {
		t.Fatal(response.Diagnostics, puts, patches)
	}
	var observed SkillResourceModel
	if d := response.State.Get(t.Context(), &observed); d.HasError() {
		t.Fatal(d)
	}
	if observed.ID.ValueString() != skillGitTestID || observed.LatestVersion.ValueInt64() != 1 || observed.Icon.ValueString() != "book" {
		t.Fatal("settings edit changed content identity/version or lost icon")
	}
}

func TestSkillGitIntervalUpdateDoesNotResendSource(t *testing.T) {
	patches := 0
	r := skillGitTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodGet {
			fmt.Fprint(w, skillGitTestDetail(`"1d"`))
			return
		}
		if req.Method != http.MethodPatch {
			t.Errorf("unexpected method %s", req.Method)
			w.WriteHeader(500)
			return
		}
		patches++
		var body map[string]json.RawMessage
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body) != 1 || string(body["interval"]) != `"1h"` {
			t.Errorf("interval update resent source or other fields: %v", body)
		}
		fmt.Fprint(w, skillGitTestDetail(`"1h"`))
	})
	before := skillGitTestObserved(t)
	after := before
	after.GitSource = skillGitTestSource(t, func(s *skillGitSourceModel) { s.SyncInterval = types.StringValue("1h") })
	state, plan := skillGitTestPlanState(t, r, before, after)
	response := resource.UpdateResponse{State: state}
	r.Update(t.Context(), resource.UpdateRequest{Plan: plan, State: state}, &response)
	if response.Diagnostics.HasError() || patches != 1 {
		t.Fatal(response.Diagnostics, patches)
	}
	var observed SkillResourceModel
	_ = response.State.Get(t.Context(), &observed)
	if !observed.GitSource.Equal(after.GitSource) || observed.ID.ValueString() != skillGitTestID {
		t.Fatal("interval edit changed identity or did not persist")
	}
}

func TestSkillGitSourceConflictFailsBeforeMutation(t *testing.T) {
	writes := 0
	r := skillGitTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodGet {
			fmt.Fprint(w, skillGitTestDetail(`"1h"`))
			return
		}
		writes++
		w.WriteHeader(500)
	})
	before := skillGitTestObserved(t)
	after := before
	after.Icon = types.StringValue("book")
	state, plan := skillGitTestPlanState(t, r, before, after)
	response := resource.UpdateResponse{State: state}
	r.Update(t.Context(), resource.UpdateRequest{Plan: plan, State: state}, &response)
	if !response.Diagnostics.HasError() || writes != 0 {
		t.Fatal("source conflict mutated the Skill", response.Diagnostics, writes)
	}
}

func TestSkillGitFailedFollowupRetainsCreatedIdentity(t *testing.T) {
	imports, puts := 0, 0
	r := skillGitTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case http.MethodPost:
			imports++
			var body map[string]json.RawMessage
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			var paths []string
			_ = json.Unmarshal(body["skillPaths"], &paths)
			if len(paths) != 1 || paths[0] != skillGitTestPath {
				t.Error("import did not select exactly one Skill path")
			}
			if _, ok := body["initialGrants"]; ok {
				t.Error("partially configured import was shared")
			}
			fmt.Fprintf(w, `{"created":[%s],"skipped":[],"skippedFiles":[]}`, skillGitTestDetail(`"1d"`))
		case http.MethodGet:
			fmt.Fprint(w, skillGitTestDetail(`"1d"`))
		case http.MethodPut:
			puts++
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{}`)
		default:
			t.Errorf("unexpected method %s", req.Method)
			w.WriteHeader(500)
		}
	})
	wanted := skillGitTestObserved(t)
	wanted.ID = types.StringUnknown()
	wanted.Icon = types.StringValue("book")
	_, plan := skillGitTestPlanState(t, r, skillGitTestObserved(t), wanted)
	var s resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &s)
	response := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(t.Context(), resource.CreateRequest{Plan: plan}, &response)
	if !response.Diagnostics.HasError() || imports != 1 || puts != 1 {
		t.Fatal(response.Diagnostics, imports, puts)
	}
	var id types.String
	if d := response.State.GetAttribute(t.Context(), path.Root("id"), &id); d.HasError() || id.ValueString() != skillGitTestID {
		t.Fatal("partial create lost its cleanup identity", d)
	}
}

func TestSkillGitImportNeverAdoptsSkippedName(t *testing.T) {
	r := skillGitTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Error("skipped name caused another request")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"created":[],"skipped":["existing"],"skippedFiles":[]}`)
	})
	wanted := skillGitTestObserved(t)
	wanted.ID = types.StringUnknown()
	_, plan := skillGitTestPlanState(t, r, skillGitTestObserved(t), wanted)
	var s resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &s)
	response := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(t.Context(), resource.CreateRequest{Plan: plan}, &response)
	if !response.Diagnostics.HasError() || !response.State.Raw.IsNull() {
		t.Fatal("existing Skill was implicitly adopted", response.Diagnostics)
	}
}

func TestSkillGitImportSkippedFilesRetainsIdentity(t *testing.T) {
	r := skillGitTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Error("partial import was followed by another mutation")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"created":[%s],"skipped":[],"skippedFiles":[{"files":["large.bin"]}]}`, skillGitTestDetail(`"1d"`))
	})
	wanted := skillGitTestObserved(t)
	wanted.ID = types.StringUnknown()
	_, plan := skillGitTestPlanState(t, r, skillGitTestObserved(t), wanted)
	var s resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &s)
	response := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(t.Context(), resource.CreateRequest{Plan: plan}, &response)
	var id types.String
	_ = response.State.GetAttribute(t.Context(), path.Root("id"), &id)
	if !response.Diagnostics.HasError() || id.ValueString() != skillGitTestID {
		t.Fatal("omitted files were accepted or created identity was lost", response.Diagnostics)
	}
}

func TestSkillGitDisconnectedProvenanceDoesNotImplyActiveSync(t *testing.T) {
	for _, hadSource := range []bool{false, true} {
		m := skillTestModel()
		if hadSource {
			m.GitSource = skillGitTestSource(t, nil)
		}
		var d diag.Diagnostics
		m.populate(t.Context(), []byte(skillGitTestDetail(`null`)), &d)
		if d.HasError() {
			t.Fatal(d)
		}
		if !hadSource && !m.GitSource.IsNull() {
			t.Fatal("disconnected provenance was adopted as active sync")
		}
		if hadSource {
			source := skillGitSource(t.Context(), m.GitSource, &d)
			if !source.SyncInterval.IsNull() {
				t.Fatal("external disconnect was hidden from the plan")
			}
		}
	}
}

func TestSkillGitObservedCredentialsAndSelection(t *testing.T) {
	m := skillGitTestObserved(t)
	detail := strings.Replace(skillGitTestDetail(`"1d"`), `"sourceProvider":"github"`, `"sourceProvider":"github","githubPatId":"22222222-2222-4222-8222-222222222222"`, 1)
	detail = strings.Replace(detail, skillGitTestRepo+"@main:"+skillGitTestPath, skillGitTestRepo+"@other:elsewhere", 1)
	detail = strings.Replace(detail, `"githubSyncRef":"main"`, `"githubSyncRef":"other"`, 1)
	var d diag.Diagnostics
	m.populate(t.Context(), []byte(detail), &d)
	if d.HasError() {
		t.Fatal(d)
	}
	source := skillGitSource(t.Context(), m.GitSource, &d)
	if source.SkillPath.ValueString() != "elsewhere" || source.RepoURL.ValueString() != "https://github.com/"+skillGitTestRepo+"/tree/other" || source.GitHubPATID.ValueString() != "22222222-2222-4222-8222-222222222222" {
		t.Fatal("source drift was hidden", source)
	}
}

func TestSkillGitConfigOwnership(t *testing.T) {
	for _, tc := range []struct {
		name      string
		edit      func(*SkillResourceModel)
		wantError bool
	}{
		{"authored", func(m *SkillResourceModel) {}, false},
		{"missing content", func(m *SkillResourceModel) { m.Content = types.StringNull() }, true},
		{"git and manifest", func(m *SkillResourceModel) { m.GitSource = skillGitTestSource(t, nil) }, true},
		{"git only", func(m *SkillResourceModel) { skillGitOnlyConfig(m, t) }, false},
		{"provider mismatch", func(m *SkillResourceModel) {
			skillGitOnlyConfig(m, t)
			m.GitSource = skillGitTestSource(t, func(s *skillGitSourceModel) {
				s.GitLabTokenID = types.StringValue("22222222-2222-4222-8222-222222222222")
			})
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &SkillResource{}
			m := skillTestModel()
			tc.edit(&m)
			_, plan := skillGitTestPlanState(t, r, m, m)
			var response resource.ValidateConfigResponse
			r.ValidateConfig(t.Context(), resource.ValidateConfigRequest{Config: tfsdk.Config(plan)}, &response)
			if response.Diagnostics.HasError() != tc.wantError {
				t.Fatal(response.Diagnostics)
			}
		})
	}
}

func skillGitOnlyConfig(m *SkillResourceModel, t *testing.T) {
	m.GitSource = skillGitTestSource(t, nil)
	m.Name = types.StringNull()
	m.Description = types.StringNull()
	m.Content = types.StringNull()
	m.License = types.StringNull()
	m.Compatibility = types.StringNull()
	m.AgentName = types.StringNull()
	m.Templated = types.BoolNull()
	m.Metadata = types.MapNull(types.StringType)
	m.AllowedTools = types.SetNull(types.StringType)
	m.Files = types.MapNull(skillFileType)
}

func TestSkillGitURLMatches(t *testing.T) {
	ref := "feature/branch"
	if !skillGitURLMatches("https://github.com/owner/repo/tree/feature%2Fbranch", "owner/repo", &ref, "github", "https://github.com") {
		t.Fatal("encoded branch ref changed")
	}
	if !skillGitURLMatches("group/repo", "group/repo", nil, "gitlab", "https://gitlab.com") {
		t.Fatal("hostless default branch changed")
	}
	if skillGitURLMatches("https://other.invalid/owner/repo", "owner/repo", nil, "github", "https://github.com") {
		t.Fatal("different source host was treated as equivalent")
	}
}

func TestSkillAuthoredClearingInvalidatesComputedOutputs(t *testing.T) {
	r := &SkillResource{}
	before := skillTestModel()
	before.License = types.StringValue("MIT")
	before.Digest = types.StringValue("old")
	before.UpdatedAt = types.StringValue("old")
	config := before
	config.License = types.StringNull()
	state, plan := skillGitTestPlanState(t, r, before, before)
	_, configured := skillGitTestPlanState(t, r, config, config)
	response := resource.ModifyPlanResponse{Plan: plan}
	r.ModifyPlan(t.Context(), resource.ModifyPlanRequest{State: state, Plan: plan, Config: tfsdk.Config(configured)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var result SkillResourceModel
	if d := response.Plan.Get(t.Context(), &result); d.HasError() {
		t.Fatal(d)
	}
	if !result.License.IsNull() || !result.Digest.IsUnknown() || !result.UpdatedAt.IsUnknown() || result.ID.ValueString() != skillGitTestID {
		t.Fatal("authored option clearing retained stale output values")
	}
}
