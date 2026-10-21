package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Tests use the tested release from the repository, as the binary does.
func init() {
	raw, err := os.ReadFile("../../platform-version.txt")
	if err != nil {
		panic(err)
	}
	if err := SetTestedAPIVersion(string(raw)); err != nil {
		panic(err)
	}
}

func TestAPICompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, body, constraint string
		status                 int
		wantError, wantWarning bool
	}{
		{"previous-certified-release-now-below-floor", `{"version":"1.4.0-rc.49"}`, "", 200, true, false},
		{"below-floor", `{"version":"1.4.0-rc.48"}`, "", 200, true, false},
		{"lacks-unauthenticated-proxy-setting", `{"version":"1.4.0-rc.50"}`, "", 200, true, false},
		{"old-stable", `{"version":"1.3.0"}`, "", 200, true, false},
		{"tested-release", fmt.Sprintf(`{"version":%q}`, testedAPIVersion), "", 200, false, false},
		{"floor", `{"version":"1.4.0-rc.51"}`, "", 200, false, true},
		{"stable-not-yet-tested", `{"version":"1.4.0"}`, "", 200, false, true},
		{"major-not-tested", `{"version":"2.0.0"}`, "", 200, false, true},
		{"build-metadata", `{"version":"1.4.0-rc.51+build.123"}`, "", 200, false, true},
		{"custom", `{"version":"dev"}`, "", 200, false, true},
		{"missing", `{}`, "", 200, false, true},
		{"malformed", `not JSON secret-marker`, "", 200, false, true},
		{"unavailable", `secret-marker`, "", 503, false, true},
		{"denied", `secret-marker`, "", 403, false, true},
		{"exact-constraint", `{"version":"1.4.0-rc.51"}`, "= 1.4.0-rc.51", 200, false, true},
		{"constraint-mismatch", `{"version":"1.4.0-rc.51"}`, "= 1.4.0-rc.52", 200, true, false},
		{"constraint-range", `{"version":"1.4.1"}`, ">= 1.4.0, < 1.5.0", 200, false, true},
		{"constraint-cannot-lower-floor", `{"version":"1.3.0"}`, ">= 1.0.0", 200, true, false},
		{"constraint-requires-version", `{"version":"dev"}`, ">= 1.4.0", 200, true, false},
		{"constraint-requires-health", `secret-marker`, ">= 1.4.0", 404, true, false},
		{"invalid-constraint", `{"version":"1.4.0-rc.49"}`, "not-a-constraint", 200, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name != "tested-release" && strings.Contains(tc.body, fmt.Sprintf("%q", testedAPIVersion)) {
				t.Skipf("%s is the tested release, so this case has no untested-version warning", testedAPIVersion)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/health" {
					t.Errorf("unexpected resource operation: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "" {
					t.Error("public health probe leaked API key")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			d := checkAPICompatibility(t.Context(), server.Client(), server.URL, "3.0.0", tc.constraint)
			if d.HasError() != tc.wantError || (d.WarningsCount() > 0) != tc.wantWarning {
				t.Fatalf("diagnostics: %v", d)
			}
			if strings.Contains(fmt.Sprint(d), "secret-marker") {
				t.Fatal("raw response body leaked into diagnostics")
			}
			if tc.name == "invalid-constraint" && calls != 0 {
				t.Fatal("invalid constraint should fail before HTTP")
			}
		})
	}
}

func TestAPICompatibilityDoesNotFollowRedirects(t *testing.T) {
	followed := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	d := checkAPICompatibility(t.Context(), server.Client(), server.URL, "3.0.0", "")
	if followed || d.WarningsCount() != 1 {
		t.Fatalf("followed=%v diagnostics=%v", followed, d)
	}
}

func TestAPICompatibilityStopsProviderConfiguration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"version":"1.4.0-rc.48"}`) }))
	defer server.Close()
	p := &ArchestraProvider{version: "3.0.0"}
	var schema provider.SchemaResponse
	p.Schema(t.Context(), provider.SchemaRequest{}, &schema)
	config := tfsdk.Config{Schema: schema.Schema}
	state := tfsdk.State{Schema: schema.Schema}
	value := ArchestraProviderModel{BaseURL: types.StringValue(server.URL), APIKey: types.StringValue("test-secret"), APIVersionConstraint: types.StringNull()}
	if d := state.Set(t.Context(), &value); d.HasError() {
		t.Fatal(d)
	}
	config.Raw = state.Raw
	var response provider.ConfigureResponse
	p.Configure(t.Context(), provider.ConfigureRequest{Config: config}, &response)
	if !response.Diagnostics.HasError() || response.ResourceData != nil || response.DataSourceData != nil {
		t.Fatalf("incompatible server configured: %+v", response)
	}
}

// The supported interval has one tested member: the release in
// platform-version.txt. Every acceptance job must run against that file, so a
// pull request that changes it tests the release the provider will report.
func TestAPICompatibilityCITestsPlatformVersion(t *testing.T) {
	raw, err := os.ReadFile("../../platform-version.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`\A\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?\n\z`).Match(raw) {
		t.Fatalf("platform-version.txt must hold one release version and a newline, got %q", raw)
	}
	workflow, err := os.ReadFile("../../.github/workflows/on-pull-request.yml")
	if err != nil {
		t.Fatal(err)
	}
	if pin := regexp.MustCompile(`ARCHESTRA_VERSION: "|archestra/platform:\d`).Find(workflow); pin != nil {
		t.Fatalf("acceptance CI hard-codes a platform release (%s); read platform-version.txt instead", pin)
	}
	jobs := regexp.MustCompile(`(?m)^  [a-z-]*acceptance-tests:\n((?:    .*\n|\n)*)`).FindAllSubmatch(workflow, -1)
	if len(jobs) == 0 {
		t.Fatal("no acceptance job found")
	}
	for _, job := range jobs {
		if !strings.Contains(string(job[1]), "./scripts/platform-version.sh") {
			t.Fatalf("acceptance job does not read platform-version.txt through scripts/platform-version.sh:\n%s", job[0])
		}
	}
	out, err := exec.CommandContext(t.Context(), "../../scripts/platform-version.sh").Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := "ARCHESTRA_VERSION=" + testedAPIVersion + "\n"; string(out) != want {
		t.Fatalf("scripts/platform-version.sh printed %q, want %q", out, want)
	}
}

func TestNightlyAcceptanceTestsSupportedReleases(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is not installed")
	}
	// A stub gh returns the stable release versions; the newest one wins.
	bin := t.TempDir()
	stub := "#!/bin/sh\nprintf '1.4.2\\n1.4.10\\n1.3.99\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o700); err != nil { //nolint:gosec // G306: the stub gh must be executable.
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "../../scripts/nightly-platform-versions.sh")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := strings.CutPrefix(strings.TrimSpace(string(out)), "matrix=")
	if !ok {
		t.Fatalf("script printed %q, want a matrix= output", out)
	}
	var matrix struct {
		Include []struct{ Version, Role string }
	}
	if err := json.Unmarshal([]byte(raw), &matrix); err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{}
	for _, entry := range matrix.Include {
		roles[entry.Version] = entry.Role
	}
	want := map[string]string{minimumAPIVersion: "oldest supported", "1.4.10": "newest stable"}
	if role, ok := want[testedAPIVersion]; ok {
		want[testedAPIVersion] = role + ", tested"
	} else {
		want[testedAPIVersion] = "tested"
	}
	if !reflect.DeepEqual(roles, want) {
		t.Fatalf("nightly releases = %v, want %v", roles, want)
	}
}
