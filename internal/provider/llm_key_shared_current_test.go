package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"os"
	"testing"
)

func TestAccLLMProviderKeySharedOwnership(t *testing.T) {
	base := os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL")
	config := func(shared string) string {
		return fmt.Sprintf(`
resource "archestra_llm_provider_api_key" "test" {
 name = "tf-shared-key-fixture"
 llm_provider = "ollama"
 base_url = %q
 %s
 %s
}
`, base, testAccLLMKeyMaterial(), shared)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		testAccPreCheck(t)
		if base == "" {
			t.Fatal("ARCHESTRA_TEST_LLM_BASE_URL must point to an isolated model fixture")
		}
	}, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("shared=true"), Check: resource.TestCheckResourceAttr("archestra_llm_provider_api_key.test", "scope", "org")},
		{ResourceName: "archestra_llm_provider_api_key.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"api_key", "vault_secret_path", "vault_secret_key"}},
		{Config: config(""), PlanOnly: true},
		{Config: config("shared=false"), Check: resource.TestCheckResourceAttr("archestra_llm_provider_api_key.test", "scope", "personal"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_llm_provider_api_key.test", plancheck.ResourceActionDestroyBeforeCreate)}}},
	}})
}
