package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestGatewayGithubActionsCreateFailureRetainsIdentity(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/api/agents" {
				t.Errorf("unexpected create: %s %s", req.Method, req.URL.Path)
			}
			if _, found := body["githubActionsAccess"]; found {
				t.Error("update-only GitHub access sent on create")
			}
			fmt.Fprintf(w, `{"id":%q,"name":"gateway","agentType":"mcp_gateway"}`, id)
		case 2:
			if req.Method != http.MethodPut || req.URL.Path != "/api/agents/"+id || len(body) != 1 || body["githubActionsAccess"] == nil {
				t.Errorf("invalid post-create patch: %s %s %v", req.Method, req.URL.Path, body)
			}
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{}`)
		default:
			t.Error("denied write retried or followed by unrelated mutation")
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := McpGatewayResource{client: c}
	var sr frameworkresource.SchemaResponse
	r.Schema(t.Context(), frameworkresource.SchemaRequest{}, &sr)
	typ, ok := sr.Schema.Type().TerraformType(t.Context()).(tftypes.Object)
	if !ok {
		t.Fatal("gateway schema must be an object")
	}
	values := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for name, fieldType := range typ.AttributeTypes {
		values[name] = tftypes.NewValue(fieldType, nil)
	}
	values["name"] = tftypes.NewValue(tftypes.String, "gateway")
	access := githubActionsAccessFromResponse([]byte(`{"githubActionsAccess":{"githubAppCredentialKey":"test-app","repositories":["owner/repo"],"workflows":["owner/repo/.github/workflows/review.yml@refs/heads/main"]}}`), &sr.Diagnostics)
	value, err := access.ToTerraformValue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	values["github_actions_access"] = value
	raw := tftypes.NewValue(typ, values)
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: sr.Schema}}
	r.Create(t.Context(), frameworkresource.CreateRequest{Plan: tfsdk.Plan{Schema: sr.Schema, Raw: raw}, Config: tfsdk.Config{Schema: sr.Schema, Raw: raw}}, &response)
	if !response.Diagnostics.HasError() || calls != 2 {
		t.Fatalf("calls=%d diagnostics=%v", calls, response.Diagnostics)
	}
	var actual McpGatewayResourceModel
	if ds := response.State.Get(t.Context(), &actual); ds.HasError() {
		t.Fatal(ds)
	}
	if actual.ID.ValueString() != id {
		t.Fatal("created gateway identity lost after denied access write", actual.ID)
	}
}

func TestGatewayGithubActionsFlattenAndClear(t *testing.T) {
	var ds diag.Diagnostics
	value := githubActionsAccessFromResponse([]byte(`{"githubActionsAccess":{"githubAppCredentialKey":"app","repositories":[],"workflows":[]}}`), &ds)
	if ds.HasError() || value.IsNull() {
		t.Fatal(value, ds)
	}
	for _, field := range []string{"repositories", "workflows"} {
		list, ok := value.Attributes()[field].(types.List)
		if !ok {
			t.Fatal("access allowlist must be a list", field)
		}
		if list.IsNull() || len(list.Elements()) != 0 {
			t.Fatal("explicit empty list lost", list)
		}
	}
	for _, body := range []string{`{}`, `{"githubActionsAccess":null}`} {
		if got := githubActionsAccessFromResponse([]byte(body), &ds); !got.IsNull() {
			t.Fatal("stale access survived refresh", got)
		}
	}
	obj := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"github_actions_access": value.Type(t.Context()).TerraformType(t.Context())}}
	priorValue, err := value.ToTerraformValue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	prior := tftypes.NewValue(obj, map[string]tftypes.Value{"github_actions_access": priorValue})
	plan := tftypes.NewValue(obj, map[string]tftypes.Value{"github_actions_access": tftypes.NewValue(obj.AttributeTypes["github_actions_access"], nil)})
	got := MergePatch(t.Context(), plan, prior, []AttrSpec{githubActionsAccessAttrSpec}, &ds)
	if !reflect.DeepEqual(got, map[string]any{"githubActionsAccess": nil}) || ds.HasError() {
		t.Fatal("removing access must explicitly disable token exchange", got, ds)
	}
}

func TestLimitRefreshReasonAndThroughput(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	body := `{"id":"` + id + `","reason":"Capacity protection","throughputUsage":{"currentUsage":7,"resetsAt":"2026-10-27T01:02:03Z"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := LimitResource{client: c}
	data := LimitResourceModel{ID: types.StringValue(id)}
	var ds diag.Diagnostics
	r.refreshCurrentLimit(t.Context(), &data, &ds)
	if ds.HasError() || data.Reason.ValueString() != "Capacity protection" || data.ThroughputUsage.IsNull() {
		t.Fatal(data, ds)
	}
	usage, ok := data.ThroughputUsage.Attributes()["current_usage"].(types.Float64)
	if !ok || usage.ValueFloat64() != 7 {
		t.Fatal("invalid throughput usage", usage)
	}
	body = `{"id":"` + id + `","reason":null}`
	r.refreshCurrentLimit(t.Context(), &data, &ds)
	if ds.HasError() || !data.Reason.IsNull() || !data.ThroughputUsage.IsNull() {
		t.Fatal("stale limit metadata survived", data, ds)
	}
}

func TestPlaywrightSettingRefreshAndPatch(t *testing.T) {
	var ds diag.Diagnostics
	data := OrganizationSettingsResourceModel{PlaywrightMcpEnabled: types.BoolValue(true)}
	flattenCurrentOrganizationSettings(&data, []byte(`{"playwrightMcpEnabled":false}`), &ds)
	if ds.HasError() || data.PlaywrightMcpEnabled.IsNull() || data.PlaywrightMcpEnabled.ValueBool() {
		t.Fatal(data.PlaywrightMcpEnabled, ds)
	}
	typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"playwright_mcp_enabled": tftypes.Bool}}
	plan := tftypes.NewValue(typ, map[string]tftypes.Value{"playwright_mcp_enabled": tftypes.NewValue(tftypes.Bool, false)})
	prior := tftypes.NewValue(typ, map[string]tftypes.Value{"playwright_mcp_enabled": tftypes.NewValue(tftypes.Bool, true)})
	patch := MergePatch(t.Context(), plan, prior, currentOrganizationSpecs("security-settings"), &ds)
	if ds.HasError() || !reflect.DeepEqual(patch, map[string]any{"playwrightMcpEnabled": false}) {
		t.Fatal(patch, ds)
	}
	flattenCurrentOrganizationSettings(&data, []byte(`{}`), &ds)
	if !data.PlaywrightMcpEnabled.IsNull() {
		t.Fatal("missing setting from older release must be null")
	}
}

func TestAccGatewayGithubActionsAccess(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.11") {
		t.Skip("GitHub Actions gateway access requires API 1.5.0-rc.11")
	}
	key := acctest.RandomWithPrefix("tf-actions")
	config := func(name, workflow string, enabled bool) string {
		access := ""
		if enabled {
			access = fmt.Sprintf(`github_actions_access={github_app_credential_key=archestra_credential.app.key,repositories=["example/repo"],workflows=[%q]}`, workflow)
		}
		return fmt.Sprintf(`resource "archestra_credential" "app" {
 key=%q
 name="Acceptance GitHub App"
 kind="github_app"
 allow_personal=false
 allow_organization=true
 app_id="12345"
 installation_id="67890"
 github_url="https://api.github.com"
}
resource "archestra_mcp_gateway" "test" {
 name=%q
 %s
}`, key, name, access)
	}
	initial := config(key, "example/repo/.github/workflows/review.yml@refs/heads/main", true)
	renamed := config(key+"-renamed", "example/repo/.github/workflows/review.yml@refs/heads/main", true)
	updated := config(key+"-renamed", "example/repo/.github/workflows/check.yml@refs/heads/main", true)
	cleared := config(key+"-renamed", "", false)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "github_actions_access.github_app_credential_key", key)},
		{ResourceName: "archestra_mcp_gateway.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: renamed, Check: resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "github_actions_access.repositories.0", "example/repo")},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "github_actions_access.workflows.0", "example/repo/.github/workflows/check.yml@refs/heads/main")},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.TestCheckNoResourceAttr("archestra_mcp_gateway.test", "github_actions_access")},
		{Config: cleared, PlanOnly: true},
	}})
}

func TestAccLimitThroughputAndReason(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.11") {
		t.Skip("Throughput limits require API 1.5.0-rc.11")
	}
	org := testAccOrganizationID(t)
	config := func(kind, reason string) string {
		return fmt.Sprintf(`resource "archestra_limit" "test" {
 entity_type="organization"
 entity_id=%q
 limit_type=%q
 limit_value=1000
 cleanup_interval="minute"
 %s
}`, org, kind, reason)
	}
	initial := config("requests", `reason="Initial protection"`)
	updated := config("requests", `reason="Updated protection"`)
	replaced := config("tokens", `reason="Updated protection"`)
	cleared := config("tokens", "")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_limit.test", "reason", "Initial protection"), resource.TestCheckResourceAttr("archestra_limit.test", "throughput_usage.current_usage", "0"))},
		{ResourceName: "archestra_limit.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"throughput_usage"}},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_limit.test", "reason", "Updated protection")},
		{Config: updated, PlanOnly: true},
		{Config: replaced, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_limit.test", plancheck.ResourceActionDestroyBeforeCreate)}}},
		{Config: replaced, PlanOnly: true},
		{Config: cleared, Check: resource.TestCheckNoResourceAttr("archestra_limit.test", "reason")},
		{Config: cleared, PlanOnly: true},
	}})
}

func TestAccOrganizationPlaywrightSetting(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.11") {
		t.Skip("Playwright MCP settings require API 1.5.0-rc.11")
	}
	disabled := `resource "archestra_organization_settings" "test" { playwright_mcp_enabled=false }`
	enabled := `resource "archestra_organization_settings" "test" { playwright_mcp_enabled=true }`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: disabled, Check: resource.TestCheckResourceAttr("archestra_organization_settings.test", "playwright_mcp_enabled", "false")},
		{ResourceName: "archestra_organization_settings.test", ImportState: true, ImportStateVerify: true},
		{Config: disabled, PlanOnly: true},
		{Config: enabled, Check: resource.TestCheckResourceAttr("archestra_organization_settings.test", "playwright_mcp_enabled", "true")},
		{Config: enabled, PlanOnly: true},
		{Config: disabled, Check: resource.TestCheckResourceAttr("archestra_organization_settings.test", "playwright_mcp_enabled", "false")},
		{Config: disabled, PlanOnly: true},
	}})
}

func TestLimitThroughputConfiguration(t *testing.T) {
	r := LimitResource{}
	var sr frameworkresource.SchemaResponse
	r.Schema(t.Context(), frameworkresource.SchemaRequest{}, &sr)
	typ, ok := sr.Schema.Type().TerraformType(t.Context()).(tftypes.Object)
	if !ok {
		t.Fatal("limit schema must be an object")
	}
	for _, tc := range []struct {
		name, kind, scope, interval string
		value                       int64
		invalid                     bool
	}{
		{"all-model zero budget", "token_cost", "organization", "24h", 0, false},
		{"requests", "requests", "organization", "minute", 1, false},
		{"tokens", "tokens", "team", "calendar_month", 1, false},
		{"virtual key", "requests", "virtual_key", "minute", 1, false},
		{"agent throughput", "tokens", "agent", "minute", 1, true},
		{"zero throughput", "requests", "organization", "minute", 0, true},
		{"zero minute budget", "token_cost", "team", "minute", 0, true},
		{"legacy rate interval", "requests_per_minute", "organization", "24h", 1, true},
		{"minute MCP calls", "mcp_server_calls", "team", "minute", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := make(map[string]tftypes.Value, len(typ.AttributeTypes))
			for name, fieldType := range typ.AttributeTypes {
				values[name] = tftypes.NewValue(fieldType, nil)
			}
			for name, value := range map[string]string{"limit_type": tc.kind, "entity_type": tc.scope, "cleanup_interval": tc.interval, "entity_id": "example"} {
				values[name] = tftypes.NewValue(tftypes.String, value)
			}
			values["limit_value"] = tftypes.NewValue(tftypes.Number, tc.value)
			var result frameworkresource.ValidateConfigResponse
			r.ValidateConfig(t.Context(), frameworkresource.ValidateConfigRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: tftypes.NewValue(typ, values)}}, &result)
			if result.Diagnostics.HasError() != tc.invalid {
				t.Fatal("unexpected limit validation", result.Diagnostics)
			}
		})
	}
}

func TestLimitDimensionReplacement(t *testing.T) {
	for _, tc := range []struct {
		prior, next string
		replace     bool
	}{
		{"requests", "requests", false}, {"requests", "tokens", true},
		{"token_cost", "requests", true}, {"tool_calls", "mcp_server_calls", true},
		{"requests_per_minute", "requests", false}, {"tokens_per_minute", "tokens", false},
		{"requests", "requests_per_minute", true},
	} {
		var response stringplanmodifier.RequiresReplaceIfFuncResponse
		limitDimensionRequiresReplace(t.Context(), planmodifier.StringRequest{StateValue: types.StringValue(tc.prior), PlanValue: types.StringValue(tc.next)}, &response)
		if response.RequiresReplace != tc.replace {
			t.Errorf("%s -> %s: replacement=%t", tc.prior, tc.next, response.RequiresReplace)
		}
	}
}
