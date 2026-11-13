package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const (
	batteryFixtureCatalogID = "33333333-3333-4333-8333-333333333333"
	batteryFixtureInstallID = "44444444-4444-4444-8444-444444444444"
)

// batteryAPIFake models the battery-installs API: a create answers with the
// battery (no install id), and the install appears in the batteries list.
type batteryAPIFake struct {
	mu      sync.Mutex
	install map[string]any
	patches []bool
	creates []map[string]any
}

func (f *batteryAPIFake) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	installPath := "/api/openappa/battery-installs/" + batteryFixtureInstallID
	switch {
	case req.Method == http.MethodPost && req.URL.Path == "/api/openappa/battery-installs":
		var body map[string]any
		_ = json.NewDecoder(req.Body).Decode(&body)
		f.creates = append(f.creates, body)
		catalogID, _ := body["catalogId"].(string)
		var catalog any
		if catalogID != "" {
			catalog = catalogID
		}
		f.install = map[string]any{
			"id": batteryFixtureInstallID, "organizationId": "org", "batteryName": body["batteryName"],
			"catalogId": catalog, "enabled": true, "status": "active", "target": nil, "packageHash": body["packageHash"],
			"lastError": nil, "credentialBindings": map[string]string{"GITHUB_TOKEN": "cred-1"},
			"createdAt": "2026-11-01T10:00:00.000Z", "updatedAt": "2026-11-01T10:00:00.000Z",
		}
		fmt.Fprintf(w, `{"name":%q,"installs":[]}`, body["batteryName"])
	case req.Method == http.MethodGet && req.URL.Path == "/api/openappa/batteries":
		installs := []any{}
		if f.install != nil {
			installs = append(installs, f.install)
		}
		// A sibling install for another catalog must not be picked.
		installs = append(installs, map[string]any{"id": "55555555-5555-4555-8555-555555555555", "batteryName": "github", "catalogId": "66666666-6666-4666-8666-666666666666"})
		writeBatteryJSON(w, []any{map[string]any{"name": "github", "installs": installs}})
	case req.URL.Path == installPath && f.install == nil:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"message":"Battery install not found","type":"api_not_found_error"}}`)
	case req.Method == http.MethodGet && req.URL.Path == installPath:
		writeBatteryJSON(w, f.install)
	case req.Method == http.MethodPatch && req.URL.Path == installPath:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		f.patches = append(f.patches, body.Enabled)
		f.install["enabled"] = body.Enabled
		f.install["updatedAt"] = "2026-11-02T10:00:00.000Z"
		fmt.Fprint(w, `{"name":"github","installs":[]}`)
	case req.Method == http.MethodDelete && req.URL.Path == installPath:
		f.install = nil
		fmt.Fprint(w, `{"success":true}`)
	default:
		// The test fails on the status; the body stays constant so no request
		// data is echoed back.
		http.Error(w, "unexpected request", http.StatusTeapot)
	}
}

func writeBatteryJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

func batteryProviderConfig(url, body string) string {
	return fmt.Sprintf(`provider "archestra" {
  base_url = %q
  api_key  = "fixture-key"
}
resource "archestra_openappa_battery_install" "test" {
%s
}
`, url, body)
}

func TestOpenappaBatteryInstallLifecycle(t *testing.T) {
	fake := &batteryAPIFake{}
	server := httptest.NewServer(fake)
	defer server.Close()
	enabledConfig := batteryProviderConfig(server.URL, fmt.Sprintf(`  battery_name = "github"
  catalog_id   = %q`, batteryFixtureCatalogID))
	disabledConfig := batteryProviderConfig(server.URL, fmt.Sprintf(`  battery_name = "github"
  catalog_id   = %q
  enabled      = false`, batteryFixtureCatalogID))
	name := "archestra_openappa_battery_install.test"
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: enabledConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr(name, "id", batteryFixtureInstallID),
					tfresource.TestCheckResourceAttr(name, "enabled", "true"),
					tfresource.TestCheckResourceAttr(name, "status", "active"),
					tfresource.TestCheckNoResourceAttr(name, "target"),
					tfresource.TestCheckNoResourceAttr(name, "package_hash"),
					tfresource.TestCheckResourceAttr(name, "credential_bindings.GITHUB_TOKEN", "cred-1"),
					tfresource.TestCheckResourceAttr(name, "created_at", "2026-11-01T10:00:00Z"),
				),
			},
			// The refreshed install matches the configuration: no phantom diff.
			{Config: enabledConfig, ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// Disabling detaches in place instead of replacing the install.
			{
				Config:           disabledConfig,
				ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)}},
				Check:            tfresource.TestCheckResourceAttr(name, "enabled", "false"),
			},
			{ResourceName: name, ImportState: true, ImportStateVerify: true},
			// Removing enabled from configuration re-attaches through the default.
			{Config: enabledConfig, Check: tfresource.TestCheckResourceAttr(name, "enabled", "true")},
			// Another catalog replaces the install.
			{
				Config: batteryProviderConfig(server.URL, `  battery_name = "github"
  catalog_id   = "77777777-7777-4777-8777-777777777777"`),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionDestroyBeforeCreate)}},
				Check:            tfresource.TestCheckResourceAttr(name, "catalog_id", "77777777-7777-4777-8777-777777777777"),
			},
		},
	})
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.creates) != 2 || fake.creates[0]["catalogId"] != batteryFixtureCatalogID || fake.creates[0]["batteryName"] != "github" ||
		fake.creates[1]["catalogId"] != "77777777-7777-4777-8777-777777777777" {
		t.Fatalf("creates = %v, want the fixture catalog install, then its replacement", fake.creates)
	}
	if _, ok := fake.creates[0]["packageHash"]; ok {
		t.Fatalf("create sent packageHash for a bundled battery: %v", fake.creates[0])
	}
	if fmt.Sprint(fake.patches) != "[false true]" {
		t.Fatalf("patches = %v, want a detach then a re-attach", fake.patches)
	}
	if fake.install != nil {
		t.Fatal("destroy left the install behind")
	}
}

func TestOpenappaBatteryInstallCreateDisabled(t *testing.T) {
	fake := &batteryAPIFake{}
	server := httptest.NewServer(fake)
	defer server.Close()
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{{
			Config: batteryProviderConfig(server.URL, fmt.Sprintf(`  battery_name = "github"
  catalog_id   = %q
  enabled      = false`, batteryFixtureCatalogID)),
			Check: tfresource.TestCheckResourceAttr("archestra_openappa_battery_install.test", "enabled", "false"),
		}},
	})
	if fmt.Sprint(fake.patches) != "[false]" {
		t.Fatalf("patches = %v, want the create followed by one detach", fake.patches)
	}
}

func TestOpenappaBatteryInstallRemovedOutsideTerraform(t *testing.T) {
	fake := &batteryAPIFake{}
	server := httptest.NewServer(fake)
	defer server.Close()
	config := batteryProviderConfig(server.URL, fmt.Sprintf(`  battery_name = "github"
  catalog_id   = %q`, batteryFixtureCatalogID))
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.install = nil
					fake.mu.Unlock()
				},
				Config: config, PlanOnly: true, ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestOpenappaBatteryInstallValidation(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"organization battery disabled": {`  battery_name = "tagger"
  enabled      = false`, "Organization Battery Cannot Be Disabled"},
		"uppercase package hash": {`  battery_name = "tagger"
  package_hash = "` + strings.Repeat("A", 64) + `"`, "lowercase 64-character SHA-256"},
		"non-canonical catalog": {`  battery_name = "github"
  catalog_id   = "not-a-uuid"`, "lowercase canonical registry entry UUID"},
	} {
		t.Run(name, func(t *testing.T) {
			tfresource.UnitTest(t, tfresource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []tfresource.TestStep{{
					Config: batteryProviderConfig("http://127.0.0.1:1", tc.body), PlanOnly: true,
					ExpectError: regexp.MustCompile(regexp.QuoteMeta(tc.want)),
				}},
			})
		})
	}
}

func TestOpenappaBatteryInstallCreateFailureSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error":{"message":"github has no synced tools","type":"api_conflict_error"}}`)
	}))
	defer server.Close()
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{{
			Config: batteryProviderConfig(server.URL, fmt.Sprintf(`  battery_name = "github"
  catalog_id   = %q`, batteryFixtureCatalogID)),
			ExpectError: regexp.MustCompile(`(?s)Status 409: .*no synced\s+tools`),
		}},
	})
}

// TestAccOpenappaBatteryInstallResource installs a real battery. Catalog
// installs need a server with synced tools and organization installs need an
// annotator-only battery, neither of which a fresh stack has, so it opts in
// through ARCHESTRA_TEST_BATTERY_NAME (and ARCHESTRA_TEST_BATTERY_CATALOG_ID
// for a battery that governs MCP servers) on a stack with OpenAPPA enabled.
func TestAccOpenappaBatteryInstallResource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests are disabled")
	}
	if !testAccAPIVersionAtLeast(t, "1.5.0-rc.1") {
		t.Skip("Battery installs require API 1.5.0-rc.1")
	}
	battery := os.Getenv("ARCHESTRA_TEST_BATTERY_NAME")
	if battery == "" {
		t.Skip("set ARCHESTRA_TEST_BATTERY_NAME (and ARCHESTRA_TEST_BATTERY_CATALOG_ID for a catalog battery) to install a real battery")
	}
	catalogID := os.Getenv("ARCHESTRA_TEST_BATTERY_CATALOG_ID")
	body := fmt.Sprintf("  battery_name = %q\n", battery)
	if catalogID != "" {
		body += fmt.Sprintf("  catalog_id   = %q\n", catalogID)
	}
	config := fmt.Sprintf("resource \"archestra_openappa_battery_install\" \"test\" {\n%s}\n", body)
	name := "archestra_openappa_battery_install.test"
	steps := []tfresource.TestStep{
		{Config: config, Check: tfresource.ComposeAggregateTestCheckFunc(
			tfresource.TestCheckResourceAttrSet(name, "id"),
			tfresource.TestCheckResourceAttr(name, "battery_name", battery),
			tfresource.TestCheckResourceAttr(name, "enabled", "true"),
			tfresource.TestCheckResourceAttrSet(name, "status"),
		)},
		{Config: config, ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{ResourceName: name, ImportState: true, ImportStateVerify: true},
	}
	if catalogID != "" {
		disabled := fmt.Sprintf("resource \"archestra_openappa_battery_install\" \"test\" {\n%s  enabled      = false\n}\n", body)
		steps = append(steps, tfresource.TestStep{Config: disabled, Check: tfresource.TestCheckResourceAttr(name, "enabled", "false")})
	}
	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    steps,
	})
}
