package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestLongContextOverrideUpdateAndReset(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	const fields = `"long_context_threshold":1000,"long_context_price_per_million_input":"2.00","long_context_price_per_million_output":"3.00","long_context_price_per_million_cache_read":"0.10","long_context_price_per_million_cache_write":"0.20"`
	priorJSON := `{"id":"` + id + `","model_id":"llama3","llm_provider":"ollama-native",` + fields + `}`
	desiredJSON := strings.Replace(priorJSON, `"2.00"`, `"4.00"`, 1)
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodPatch {
			writes++
			var patch map[string]any
			if err := json.NewDecoder(req.Body).Decode(&patch); err != nil {
				t.Error(err)
			}
			for _, key := range []string{"longContextThreshold", "longContextPricePerMillionInput", "longContextPricePerMillionOutput", "longContextPricePerMillionCacheRead", "longContextPricePerMillionCacheWrite"} {
				value, exists := patch[key]
				if !exists {
					t.Errorf("atomic long-context member %s missing", key)
				}
				if writes == 2 && value != nil {
					t.Errorf("reset left %s set", key)
				}
			}
			if writes == 1 && (len(patch) != 5 || patch["longContextPricePerMillionInput"] != "4.00" || patch["longContextThreshold"] != float64(1000)) {
				t.Errorf("incorrect pricing patch: %v", patch)
			}
			fmt.Fprint(w, `{}`)
			return
		}
		fmt.Fprint(w, `[{"id":"`+id+`","modelId":"llama3","provider":"ollama-native","longContextThreshold":1000,"longContextPricePerMillionInput":"4.00000000","longContextPricePerMillionOutput":"3.00","longContextPricePerMillionCacheRead":"0.10","longContextPricePerMillionCacheWrite":"0.20"}]`)
	}))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	model := &LlmModelResource{client: api}
	prior := modelFixtureState(t, model, priorJSON)
	desired := modelFixtureState(t, model, desiredJSON)
	response := frameworkresource.UpdateResponse{State: prior}
	model.Update(t.Context(), frameworkresource.UpdateRequest{State: prior, Plan: tfsdk.Plan(desired)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var data LlmModelResourceModel
	if ds := response.State.Get(t.Context(), &data); ds.HasError() {
		t.Fatal(ds)
	}
	if data.LongContextPricePerMillionInput.ValueString() != "4.00" {
		t.Fatal("updated price was not refreshed", data.LongContextPricePerMillionInput)
	}
	var deleted frameworkresource.DeleteResponse
	model.Delete(t.Context(), frameworkresource.DeleteRequest{State: response.State}, &deleted)
	if deleted.Diagnostics.HasError() || writes != 2 {
		t.Fatal("reset failed", writes, deleted.Diagnostics)
	}
}

func TestRC19MetadataRefreshAndMissingFields(t *testing.T) {
	var ds diag.Diagnostics
	data := LlmModelResourceModel{}
	flattenCurrentModelFields(t.Context(), &data, json.RawMessage(`{"mediaPricing":{"image":{"price":"1.00"}},"longContextThreshold":1000}`), &ds)
	if ds.HasError() || data.MediaPricing.IsNull() || data.LongContextThreshold.ValueInt64() != 1000 {
		t.Fatal("missing model metadata", data, ds)
	}
	flattenCurrentModelFields(t.Context(), &data, json.RawMessage(`{}`), &ds)
	if ds.HasError() || !data.MediaPricing.IsNull() || !data.LongContextThreshold.IsNull() {
		t.Fatal("stale model metadata", data, ds)
	}
	org := OrganizationSettingsResourceModel{}
	flattenCurrentOrganizationSettings(&org, []byte(`{"keepMediaInLogs":true,"agentRuntimeTiers":{"defaultTier":"S","tiers":[]}}`), &ds)
	if ds.HasError() || !org.KeepMediaInLogs.ValueBool() || org.AgentRuntimeTiers.IsNull() {
		t.Fatal("missing organization metadata", org, ds)
	}
	flattenCurrentOrganizationSettings(&org, []byte(`{}`), &ds)
	if ds.HasError() || !org.KeepMediaInLogs.IsNull() || !org.AgentRuntimeTiers.IsNull() {
		t.Fatal("stale organization metadata", org, ds)
	}
	for _, body := range []string{`{"behavior":"soft","estimatedUsage":true,"teamManaged":true,"isGithubAppKey":true,"githubOrganization":"synthetic-org"}`, `{}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, body)
		}))
		api, err := client.NewClientWithResponses(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		limit, err := api.GetLimitWithResponse(t.Context(), uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		var limitData LimitResourceModel
		(&LimitResource{}).flattenCurrentLimit(t.Context(), &limitData, limit, &ds)
		key, err := api.GetLlmProviderApiKeyWithResponse(t.Context(), uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		var keyData LLMProviderApiKeyResourceModel
		(&LLMProviderApiKeyResource{}).flattenKey(t.Context(), &keyData, key, &ds)
		if ds.HasError() {
			t.Fatal(ds)
		}
		if body != "{}" {
			if limitData.Behavior.ValueString() != "soft" || !limitData.EstimatedUsage.ValueBool() || !limitData.TeamManaged.ValueBool() || !keyData.IsGithubAppKey.ValueBool() || keyData.GithubOrganization.ValueString() != "synthetic-org" {
				t.Fatal("missing limit/key metadata", limitData, keyData)
			}
		} else if !limitData.Behavior.IsNull() || !limitData.EstimatedUsage.IsNull() || !keyData.IsGithubAppKey.IsNull() || !keyData.GithubOrganization.IsNull() {
			t.Fatal("missing fields must remain null on older APIs")
		}
		server.Close()
	}
}

func TestAccPlatformRC19PricingAndLimits(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.19") {
		t.Skip("Long-context pricing and soft limits require API 1.5.0-rc.19")
	}
	baseURL := os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL")
	orgID := testAccOrganizationID(t)
	config := func(price, behavior string, keep bool) string {
		pricing := ""
		if price != "" {
			pricing = fmt.Sprintf(`long_context_threshold=1000
long_context_price_per_million_input=%q
long_context_price_per_million_output="3.00"
long_context_price_per_million_cache_read="0.10"
long_context_price_per_million_cache_write="0.20"`, price)
		}
		return fmt.Sprintf(`resource "archestra_model_provider" "rc19" {
 name="tf-rc19-provider"
 llm_provider="ollama-native"
 base_url=%q
 %s
}
resource "archestra_model" "rc19" {
 model_id="llama3"
 llm_provider="ollama-native"
 depends_on=[archestra_model_provider.rc19]
 %s
}
resource "archestra_limit" "rc19" {
 entity_type="organization"
 entity_id=%q
 limit_type="token_cost"
 limit_value=100000
 behavior=%q
}
resource "archestra_organization_settings" "rc19" {
 keep_media_in_logs=%t
}`, baseURL, testAccLLMKeyMaterial(), pricing, orgID, behavior, keep)
	}
	initial := config("2.00", "soft", true)
	updated := config("4.00", "hard", false)
	cleared := config("", "hard", false)
	resource.Test(t, resource.TestCase{PreCheck: func() {
		testAccPreCheck(t)
		if baseURL == "" {
			t.Fatal("ARCHESTRA_TEST_LLM_BASE_URL must point to the isolated Ollama mock")
		}
	}, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_model.rc19", "long_context_price_per_million_input", "2.00"), resource.TestCheckResourceAttr("archestra_limit.rc19", "behavior", "soft"), resource.TestCheckResourceAttr("archestra_organization_settings.rc19", "keep_media_in_logs", "true"), resource.TestCheckResourceAttr("archestra_model_provider.rc19", "is_github_app_key", "false"))},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_model.rc19", "long_context_price_per_million_input", "4.00"), resource.TestCheckResourceAttr("archestra_limit.rc19", "behavior", "hard"), resource.TestCheckResourceAttr("archestra_organization_settings.rc19", "keep_media_in_logs", "false"))},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.TestCheckNoResourceAttr("archestra_model.rc19", "long_context_threshold")},
		{Config: cleared, PlanOnly: true},
	}})
}
