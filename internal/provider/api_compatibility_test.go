package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAPICompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, body, constraint string
		status                 int
		wantError, wantWarning bool
	}{
		{"tested", `{"version":"1.4.0-rc.49"}`, "", 200, false, false},
		{"below-floor", `{"version":"1.4.0-rc.48"}`, "", 200, true, false},
		{"old-stable", `{"version":"1.3.0"}`, "", 200, true, false},
		{"next-rc", `{"version":"1.4.0-rc.50"}`, "", 200, false, true},
		{"stable-not-yet-tested", `{"version":"1.4.0"}`, "", 200, false, true},
		{"major-not-tested", `{"version":"2.0.0"}`, "", 200, false, true},
		{"build-metadata", `{"version":"1.4.0-rc.49+build.123"}`, "", 200, false, false},
		{"custom", `{"version":"dev"}`, "", 200, false, true},
		{"missing", `{}`, "", 200, false, true},
		{"malformed", `not JSON secret-marker`, "", 200, false, true},
		{"unavailable", `secret-marker`, "", 503, false, true},
		{"denied", `secret-marker`, "", 403, false, true},
		{"exact-constraint", `{"version":"1.4.0-rc.49"}`, "= 1.4.0-rc.49", 200, false, false},
		{"constraint-mismatch", `{"version":"1.4.0-rc.50"}`, "= 1.4.0-rc.49", 200, true, false},
		{"constraint-range", `{"version":"1.4.1"}`, ">= 1.4.0, < 1.5.0", 200, false, true},
		{"constraint-cannot-lower-floor", `{"version":"1.3.0"}`, ">= 1.0.0", 200, true, false},
		{"constraint-requires-version", `{"version":"dev"}`, ">= 1.4.0", 200, true, false},
		{"constraint-requires-health", `secret-marker`, ">= 1.4.0", 404, true, false},
		{"invalid-constraint", `{"version":"1.4.0-rc.49"}`, "not-a-constraint", 200, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
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

// The current supported interval has one certified member. CI must keep testing
// that release; a future interval expansion must add oldest/latest API jobs.
func TestAPICompatibilityCIPinsTestedRelease(t *testing.T) {
	workflow, err := os.ReadFile("../../.github/workflows/on-pull-request.yml")
	if err != nil {
		t.Fatal(err)
	}
	pins := regexp.MustCompile(`ARCHESTRA_VERSION: "([^"]+)"`).FindAllStringSubmatch(string(workflow), -1)
	if len(pins) == 0 {
		t.Fatal("acceptance CI has no pinned Archestra release")
	}
	for _, pin := range pins {
		if testedAPIConstraint != "= "+pin[1] {
			t.Fatalf("API test pin %s and tested interval %s differ; run compatibility evidence before updating both", pin[1], testedAPIConstraint)
		}
	}
}
