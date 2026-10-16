package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLLMProviderKeyCurrentFields(t *testing.T) {
	baseURL := os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL")
	if baseURL == "" {
		t.Fatal("ARCHESTRA_TEST_LLM_BASE_URL must point to the isolated Ollama mock accessible by the backend")
	}
	config := func(extra string) string {
		return fmt.Sprintf(`resource "archestra_llm_provider_api_key" "test" {
 name="tf-current-key-fields"
 llm_provider="ollama"
 api_key="synthetic-local-test-key"
 base_url=%q
 %s
}`, baseURL, extra)
	}
	initial := config(fmt.Sprintf(`inference_base_url=%q
 extra_headers={"X-Synthetic-Test"="first"}
 labels=[{key="provider-test",value="initial"}]`, baseURL))
	updated := config(fmt.Sprintf(`inference_base_url=%q
 extra_headers={"X-Synthetic-Test"="second"}
 labels=[{key="provider-test",value="updated"}]`, baseURL+"/"))
	cleared := config(`extra_headers={}
 labels=[]`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_llm_provider_api_key.test", "extra_headers.X-Synthetic-Test", "first"), resource.TestCheckResourceAttr("archestra_llm_provider_api_key.test", "labels.0.value", "initial"), resource.TestCheckResourceAttr("archestra_llm_provider_api_key.test", "requires_reauthentication", "false"))},
		{ResourceName: "archestra_llm_provider_api_key.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"api_key"}},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_llm_provider_api_key.test", "extra_headers.X-Synthetic-Test", "second")},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_llm_provider_api_key.test", "extra_headers.%", "0"), resource.TestCheckNoResourceAttr("archestra_llm_provider_api_key.test", "inference_base_url"))},
		{Config: cleared, PlanOnly: true},
	}})
}
