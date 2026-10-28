package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAgentModelPoolPlanning(t *testing.T) {
	const poolA = "11111111-1111-4111-8111-111111111111"
	const poolB = "22222222-2222-4222-8222-222222222222"
	const modelA = "33333333-3333-4333-8333-333333333333"
	const keyA = "44444444-4444-4444-8444-444444444444"
	r := AgentResource{}
	var sr frameworkresource.SchemaResponse
	r.Schema(t.Context(), frameworkresource.SchemaRequest{}, &sr)
	kind := sr.Schema.Type().TerraformType(t.Context())
	state := func(body string) tfsdk.State {
		encoded, err := projectStateJSON([]byte(body), kind)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := (tfprotov6.RawState{JSON: encoded}).Unmarshal(kind)
		if err != nil {
			t.Fatal(err)
		}
		return tfsdk.State{Schema: sr.Schema, Raw: raw}
	}
	prior := state(fmt.Sprintf(`{"id":"existing","name":"agent","model_pool_id":%q,"model_id":%q,"llm_api_key_id":%q}`, poolA, modelA, keyA))
	for _, tc := range []struct {
		name, body string
		unknown    bool
		wantModel  string
	}{
		{"stable anchors on metadata update", fmt.Sprintf(`{"name":"renamed","model_pool_id":%q}`, poolA), false, modelA},
		{"pool switch derives new anchors", fmt.Sprintf(`{"name":"agent","model_pool_id":%q}`, poolB), true, ""},
		{"pool clear clears anchors", `{"name":"agent"}`, false, ""},
		{"direct model selection", fmt.Sprintf(`{"name":"agent","model_id":%q,"llm_api_key_id":%q}`, modelA, keyA), false, modelA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configured := state(tc.body)
			plan := tfsdk.Plan{Schema: sr.Schema, Raw: configured.Raw}
			result := frameworkresource.ModifyPlanResponse{Plan: plan}
			r.ModifyPlan(t.Context(), frameworkresource.ModifyPlanRequest{State: prior, Plan: plan, Config: tfsdk.Config(configured)}, &result)
			if result.Diagnostics.HasError() {
				t.Fatal(result.Diagnostics)
			}
			var value types.String
			if ds := result.Plan.GetAttribute(t.Context(), path.Root("model_id"), &value); ds.HasError() {
				t.Fatal(ds)
			}
			if value.IsUnknown() != tc.unknown || (!tc.unknown && value.ValueString() != tc.wantModel) {
				t.Fatal("invalid derived model plan", value)
			}
			var ds diag.Diagnostics
			patch := MergePatch(t.Context(), result.Plan.Raw, prior.Raw, agentAttrSpec, &ds)
			if ds.HasError() {
				t.Fatal(ds)
			}
			if tc.unknown {
				if _, found := patch["modelId"]; found {
					t.Fatal("old model anchor sent with new pool", patch)
				}
			}
		})
	}
	var data AgentResourceModel
	var ds diag.Diagnostics
	r.flattenAgentResponse(t.Context(), &data, []byte(fmt.Sprintf(`{"id":%q,"name":"agent","modelPoolId":%q,"modelId":%q,"llmApiKeyId":%q}`, poolB, poolA, modelA, keyA)), &ds)
	if ds.HasError() || data.ModelPoolID.ValueString() != poolA || data.LlmApiKeyId.ValueString() != keyA {
		t.Fatal(data.ModelPoolID, data.LlmApiKeyId, ds)
	}
}

func TestOauthModelPoolGrantsRefreshAndClear(t *testing.T) {
	const pool = "11111111-1111-4111-8111-111111111111"
	r := OauthClientResource{}
	data := OauthClientResourceModel{ClientSecret: types.StringValue("synthetic-retained-secret")}
	var ds diag.Diagnostics
	r.flatten(t.Context(), &data, []byte(`{"createdBy":null,"modelPoolIds":["`+pool+`"]}`), &ds)
	if ds.HasError() || len(data.ModelPoolIDs.Elements()) != 1 {
		t.Fatal(data.ModelPoolIDs, ds)
	}
	r.flatten(t.Context(), &data, []byte(`{"createdBy":null,"modelPoolIds":[]}`), &ds)
	if ds.HasError() || data.ModelPoolIDs.IsNull() || len(data.ModelPoolIDs.Elements()) != 0 || data.ClientSecret.ValueString() != "synthetic-retained-secret" {
		t.Fatal("clearing pool grants lost credential or empty set", data.ModelPoolIDs, ds)
	}
}

func TestGithubActionsOptionsRoundTrip(t *testing.T) {
	var ds diag.Diagnostics
	value := githubActionsAccessFromResponse([]byte(`{"githubActionsAccess":{"githubAppCredentialKey":"app","repositories":["example/*"],"workflows":["example/repo/.github/workflows/review.yml@refs/heads/main"],"actAs":"actor","resolveReviewThreads":true}}`), &ds)
	raw, err := value.ToTerraformValue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"github_actions_access": raw.Type()}}
	plan := tftypes.NewValue(typ, map[string]tftypes.Value{"github_actions_access": raw})
	patch := MergePatch(t.Context(), plan, tftypes.NewValue(typ, nil), []AttrSpec{githubActionsAccessAttrSpec}, &ds)
	access, ok := patch["githubActionsAccess"].(map[string]any)
	if ds.HasError() || !ok || access["actAs"] != "actor" || access["resolveReviewThreads"] != true {
		t.Fatal("GitHub options were not preserved atomically", patch, ds)
	}
}

func testAccModelPoolFixtures(t *testing.T) (string, string, string, string) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		return "", "", "", ""
	}
	if !testAccAPIVersionAtLeast(t, "1.5.0-rc.12") {
		t.Skip("Model-pool grants require API 1.5.0-rc.12")
	}
	testAccPreCheck(t)
	parsed, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
	if err != nil || parsed.Scheme != "http" || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("Model-pool fixtures require an isolated loopback API")
	}
	transport := http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(ctx context.Context, method, endpoint string, payload any) map[string]any {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		// #nosec G704 -- The API URL is validated as loopback HTTP and redirects are refused.
		request, err := http.NewRequestWithContext(ctx, method, parsed.String()+endpoint, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		request.Header.Set("Content-Type", "application/json")
		// #nosec G704 -- The API URL is validated as loopback HTTP and redirects are refused.
		response, err := transport.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != 200 && response.StatusCode != 201 && response.StatusCode != 204 {
			var problem struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.NewDecoder(response.Body).Decode(&problem)
			t.Fatalf("isolated fixture %s %s returned %d: %s", method, endpoint, response.StatusCode, problem.Error.Message)
		}
		var result map[string]any
		if response.StatusCode != 204 {
			if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
		}
		return result
	}
	name := acctest.RandomWithPrefix("tf-pool")
	pools, keys := []string{}, []string{}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		for _, id := range pools {
			call(ctx, http.MethodDelete, "/api/model-pools/"+id, nil)
		}
		for _, id := range keys {
			call(ctx, http.MethodDelete, "/api/llm-provider-api-keys/"+id, nil)
		}
	})
	for i := 0; i < 2; i++ {
		key := call(t.Context(), http.MethodPost, "/api/llm-provider-api-keys", map[string]any{"name": fmt.Sprintf("%s-key-%d", name, i), "provider": "ollama", "baseUrl": os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL"), "apiKey": "synthetic-pool-key", "isPrimary": false, "shared": true, "initialGrants": []map[string]any{{"subject": map[string]any{"type": "role", "id": "admin"}, "actions": []string{"read", "use"}}}})
		keyID, ok := key["id"].(string)
		if !ok {
			t.Fatal("missing fixture key ID")
		}
		keys = append(keys, keyID)
		pool := call(t.Context(), http.MethodPost, "/api/model-pools", map[string]any{"name": fmt.Sprintf("%s-%d", name, i), "members": []map[string]any{{"providerApiKeyId": keyID, "modelId": "llama3"}}})
		poolID, ok := pool["id"].(string)
		if !ok {
			t.Fatal("missing fixture pool ID")
		}
		pools = append(pools, poolID)
	}
	return pools[0], pools[1], keys[0], keys[1]
}

func TestAccAgentAndOauthModelPoolLifecycle(t *testing.T) {
	first, second, firstKey, secondKey := testAccModelPoolFixtures(t)
	config := func(pool string) string {
		binding := ""
		if pool != "" {
			binding = fmt.Sprintf("model_pool_id=%q", pool)
		}
		grants := "[]"
		if pool != "" {
			grants = fmt.Sprintf("[%q]", pool)
		}
		return fmt.Sprintf(`resource "archestra_agent" "pool" {
 name="tf-model-pool-agent"
 %s
}
resource "archestra_oauth_client" "pool" {
 name="tf-model-pool-client"
 scopes=["llm:proxy"]
 model_pool_ids=%s
 provider_api_keys=[{provider="ollama",provider_api_key_id=%q}]
}`, binding, grants, firstKey)
	}
	initial, updated, cleared := config(first), config(second), config("")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_agent.pool", "model_pool_id", first), resource.TestCheckResourceAttr("archestra_agent.pool", "llm_api_key_id", firstKey), resource.TestCheckResourceAttr("archestra_oauth_client.pool", "model_pool_ids.#", "1"))},
		{ResourceName: "archestra_agent.pool", ImportState: true, ImportStateVerify: true},
		{ResourceName: "archestra_oauth_client.pool", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"client_secret"}},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_agent.pool", "model_pool_id", second), resource.TestCheckResourceAttr("archestra_agent.pool", "llm_api_key_id", secondKey))},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckNoResourceAttr("archestra_agent.pool", "model_pool_id"), resource.TestCheckNoResourceAttr("archestra_agent.pool", "model_id"), resource.TestCheckNoResourceAttr("archestra_agent.pool", "llm_api_key_id"), resource.TestCheckResourceAttr("archestra_oauth_client.pool", "model_pool_ids.#", "0"))},
		{Config: cleared, PlanOnly: true},
	}})
}

func TestAccGatewayGithubActionsRc12Options(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.12") {
		t.Skip("GitHub Actions actor and review-thread options require API 1.5.0-rc.12")
	}
	key := acctest.RandomWithPrefix("tf-actions-options")
	config := func(options string) string {
		return fmt.Sprintf(`resource "archestra_credential" "app" {
 key=%q
 name="Synthetic acceptance GitHub App"
 kind="github_app"
 allow_personal=false
 allow_organization=true
 app_id="12345"
 installation_id="67890"
 github_url="https://api.github.com"
}
resource "archestra_mcp_gateway" "options" {
 name=%q
 github_actions_access={
 github_app_credential_key=archestra_credential.app.key
 repositories=["example/*"]
 workflows=["example/repo/.github/workflows/review.yml@refs/heads/main"]
 %s
 }
}`, key, key, options)
	}
	initial := config("act_as=\"actor\"\nresolve_review_threads=true")
	updated := config("act_as=\"author\"\nresolve_review_threads=false")
	cleared := config("")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_mcp_gateway.options", "github_actions_access.act_as", "actor"), resource.TestCheckResourceAttr("archestra_mcp_gateway.options", "github_actions_access.resolve_review_threads", "true"))},
		{ResourceName: "archestra_mcp_gateway.options", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_mcp_gateway.options", "github_actions_access.act_as", "author"), resource.TestCheckResourceAttr("archestra_mcp_gateway.options", "github_actions_access.resolve_review_threads", "false"))},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckNoResourceAttr("archestra_mcp_gateway.options", "github_actions_access.act_as"), resource.TestCheckNoResourceAttr("archestra_mcp_gateway.options", "github_actions_access.resolve_review_threads"))},
		{Config: cleared, PlanOnly: true},
	}})
}
