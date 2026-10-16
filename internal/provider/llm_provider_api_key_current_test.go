package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLLMProviderKeyCurrentFields(t *testing.T) {
	baseURL := os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL")
	if os.Getenv("TF_ACC") != "" && baseURL == "" {
		t.Fatal("ARCHESTRA_TEST_LLM_BASE_URL must point to the isolated Ollama mock accessible by the backend")
	}
	config := func(extra string) string {
		return fmt.Sprintf(`resource "archestra_model_provider" "test" {
 name="tf-current-key-fields"
 llm_provider="ollama"
 %s
 base_url=%q
 %s
}`, testAccLLMKeyMaterial(), baseURL, extra)
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
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_model_provider.test", "extra_headers.X-Synthetic-Test", "first"), resource.TestCheckResourceAttr("archestra_model_provider.test", "labels.0.value", "initial"), resource.TestCheckResourceAttr("archestra_model_provider.test", "requires_reauthentication", "false"))},
		{ResourceName: "archestra_model_provider.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"api_key", "vault_secret_path", "vault_secret_key"}},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_model_provider.test", "extra_headers.X-Synthetic-Test", "second")},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_model_provider.test", "extra_headers.%", "0"), resource.TestCheckNoResourceAttr("archestra_model_provider.test", "inference_base_url"))},
		{Config: cleared, PlanOnly: true},
	}})
}

// The same resource contract is exercised against both supported secret stores.
func testAccLLMKeyMaterial() string {
	if os.Getenv("ARCHESTRA_READONLY_VAULT_ENABLED") == "true" {
		return `vault_secret_path="secret/data/test/ollama"
 vault_secret_key="api_key"`
	}
	return `api_key="synthetic-local-test-key"`
}
