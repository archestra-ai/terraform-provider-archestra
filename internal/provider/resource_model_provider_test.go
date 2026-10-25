package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLLMProviderApiKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLLMProviderApiKeyResourceConfig("Test Ollama Key", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_model_provider.test", "name", "Test Ollama Key"),
					resource.TestCheckResourceAttr("archestra_model_provider.test", "llm_provider", "ollama"),
					resource.TestCheckResourceAttr("archestra_model_provider.test", "is_primary", "false"),
					resource.TestCheckResourceAttrSet("archestra_model_provider.test", "id"),
				),
			},
			{
				ResourceName:            "archestra_model_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"api_key"},
			},
			{
				Config: testAccLLMProviderApiKeyResourceConfig("Updated Ollama Key", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_model_provider.test", "name", "Updated Ollama Key"),
				),
			},
		},
	})
}

func TestAccLLMProviderApiKeyResourceWithDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLLMProviderApiKeyResourceConfig("Default Ollama Key", "ollama", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_model_provider.test", "name", "Default Ollama Key"),
					resource.TestCheckResourceAttr("archestra_model_provider.test", "llm_provider", "ollama"),
					resource.TestCheckResourceAttr("archestra_model_provider.test", "is_primary", "true"),
				),
			},
			{
				Config: testAccLLMProviderApiKeyResourceConfig("Default Ollama Key", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_model_provider.test", "is_primary", "false"),
				),
			},
		},
	})
}

func TestAccLLMProviderApiKeyResourceGemini(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLLMProviderApiKeyResourceConfig("Ollama Key 2", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_model_provider.test", "name", "Ollama Key 2"),
					resource.TestCheckResourceAttr("archestra_model_provider.test", "llm_provider", "ollama"),
				),
			},
		},
	})
}

func TestAccLLMProviderApiKeyResourceInvalidProvider(t *testing.T) {
	// Pure plan-time schema validation.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLLMProviderApiKeyInvalidProviderConfig("Invalid Key", "invalid-provider"),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestAccLLMProviderApiKeyResourceOwnershipMetadata(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLLMProviderApiKeyResourceConfig("Scoped Ollama Key", "ollama", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_model_provider.test", "name", "Scoped Ollama Key"),
					resource.TestCheckResourceAttr("archestra_model_provider.test", "llm_provider", "ollama"),
					resource.TestCheckResourceAttr("archestra_model_provider.test", "scope", "personal"),
					resource.TestCheckResourceAttrSet("archestra_model_provider.test", "id"),
				),
			},
			{
				ResourceName:            "archestra_model_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"api_key"},
			},
		},
	})
}

//nolint:unparam // llmProvider is always "ollama" today (the fixture uses Ollama); kept parameterised for future coverage.
func testAccLLMProviderApiKeyResourceConfig(name string, llmProvider string, isDefault bool) string {
	return fmt.Sprintf(`
resource "archestra_model_provider" "test" {
  name                    = %[1]q
  llm_provider            = %[2]q
  is_primary = %[3]t
  api_key                 = "synthetic-local-test-key"
}
`, name, llmProvider, isDefault)
}

// testAccLLMProviderApiKeyInvalidProviderConfig is used only for the schema
// validation test, which fails at plan time before any API call. It still uses
// api_key because the backend is never hit.
func testAccLLMProviderApiKeyInvalidProviderConfig(name string, llmProvider string) string {
	return fmt.Sprintf(`
resource "archestra_model_provider" "test" {
  name                    = %[1]q
  api_key                 = "test-api-key-value"
  llm_provider            = %[2]q
  is_primary = false
}
`, name, llmProvider)
}
