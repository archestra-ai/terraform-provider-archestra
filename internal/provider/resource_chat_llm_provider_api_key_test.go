package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccChatLLMProviderApiKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireByosEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Test Ollama Key", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Test Ollama Key"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "llm_provider", "ollama"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "is_organization_default", "false"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "vault_secret_path", "secret/data/test/ollama"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "vault_secret_key", "api_key"),
					resource.TestCheckResourceAttrSet("archestra_chat_llm_provider_api_key.test", "id"),
				),
			},
			{
				ResourceName:            "archestra_chat_llm_provider_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"vault_secret_path", "vault_secret_key"},
			},
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Updated Ollama Key", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Updated Ollama Key"),
				),
			},
		},
	})
}

func TestAccChatLLMProviderApiKeyResourceWithDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireByosEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Default Ollama Key", "ollama", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Default Ollama Key"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "llm_provider", "ollama"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "is_organization_default", "true"),
				),
			},
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Default Ollama Key", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "is_organization_default", "false"),
				),
			},
		},
	})
}

func TestAccChatLLMProviderApiKeyResourceGemini(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireByosEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChatLLMProviderApiKeyResourceConfig("Ollama Key 2", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Ollama Key 2"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "llm_provider", "ollama"),
				),
			},
		},
	})
}

func TestAccChatLLMProviderApiKeyResourceInvalidProvider(t *testing.T) {
	// Pure plan-time schema validation; does not hit the backend so no BYOS gate.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccChatLLMProviderApiKeyInvalidProviderConfig("Invalid Key", "invalid-provider"),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestAccChatLLMProviderApiKeyResourceWithScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireByosEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChatLLMProviderApiKeyResourceConfigWithScope("Scoped Ollama Key", "ollama", "org"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "name", "Scoped Ollama Key"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "llm_provider", "ollama"),
					resource.TestCheckResourceAttr("archestra_chat_llm_provider_api_key.test", "scope", "org"),
					resource.TestCheckResourceAttrSet("archestra_chat_llm_provider_api_key.test", "id"),
				),
			},
			{
				ResourceName:            "archestra_chat_llm_provider_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"vault_secret_path", "vault_secret_key"},
			},
		},
	})
}

func testAccChatLLMProviderApiKeyResourceConfigWithScope(name string, llmProvider string, scope string) string {
	return fmt.Sprintf(`
resource "archestra_chat_llm_provider_api_key" "test" {
  name                    = %[1]q
  llm_provider            = %[2]q
  is_organization_default = false
  scope                   = %[3]q
  vault_secret_path       = "secret/data/test/ollama"
  vault_secret_key        = "api_key"
}
`, name, llmProvider, scope)
}

//nolint:unparam // llmProvider is always "ollama" today (BYOS tests seed a single vault secret); kept parameterised for future coverage.
func testAccChatLLMProviderApiKeyResourceConfig(name string, llmProvider string, isDefault bool) string {
	return fmt.Sprintf(`
resource "archestra_chat_llm_provider_api_key" "test" {
  name                    = %[1]q
  llm_provider            = %[2]q
  is_organization_default = %[3]t
  vault_secret_path       = "secret/data/test/ollama"
  vault_secret_key        = "api_key"
}
`, name, llmProvider, isDefault)
}

// testAccChatLLMProviderApiKeyInvalidProviderConfig is used only for the schema
// validation test, which fails at plan time before any API call. It still uses
// api_key because the backend is never hit.
func testAccChatLLMProviderApiKeyInvalidProviderConfig(name string, llmProvider string) string {
	return fmt.Sprintf(`
resource "archestra_chat_llm_provider_api_key" "test" {
  name                    = %[1]q
  api_key                 = "test-api-key-value"
  llm_provider            = %[2]q
  is_organization_default = false
}
`, name, llmProvider)
}
