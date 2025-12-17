package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccChatLLMProviderApiKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Test OpenAI Key", "openai", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Test OpenAI Key"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "llm_provider", "openai"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "is_organization_default", "false"),
					resource.TestCheckResourceAttrSet("archestra_chat_llm_provider_api_key.test", "id"),
				),
			},
			{
				ResourceName:            "archestra_chat_llm_provider_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"api_key"},
			},
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Updated OpenAI Key", "openai", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Updated OpenAI Key"),
				),
			},
		},
	})
}

func TestAccChatLLMProviderApiKeyResourceWithDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Default Anthropic Key", "anthropic", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Default Anthropic Key"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "llm_provider", "anthropic"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "is_organization_default", "true"),
				),
			},
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Default Anthropic Key", "anthropic", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "is_organization_default", "false"),
				),
			},
		},
	})
}

func TestAccChatLLMProviderApiKeyResourceGemini(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Gemini Key", "gemini", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Gemini Key"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "llm_provider", "gemini"),
				),
			},
		},
	})
}

func TestAccChatLLMProviderApiKeyResourceInvalidProvider(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccChatLLMProviderApiKeyResourceConfig("Invalid Key", "invalid-provider", false),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func testAccChatLLMProviderApiKeyResourceConfig(name string, llmProvider string, isDefault bool) string {
	return fmt.Sprintf(`
resource "archestra_chat_llm_provider_api_key" "test" {
  name                    = %[1]q
  api_key                 = "test-api-key-value"
  llm_provider            = %[2]q
  is_organization_default = %[3]t
}
`, name, llmProvider, isDefault)
}
