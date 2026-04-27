package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccGetFirstModelID discovers the first available LLM model from the backend.
// Returns the model_id string. Skips the test if no models are available.
func testAccGetFirstModelID(t *testing.T) string {
	t.Helper()

	baseURL := os.Getenv("ARCHESTRA_BASE_URL")
	apiKey := os.Getenv("ARCHESTRA_API_KEY")

	c, err := client.NewClientWithResponses(baseURL, client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", apiKey)
		return nil
	}))
	if err != nil {
		t.Fatalf("Unable to create client: %s", err)
	}

	resp, err := c.GetModelsWithApiKeysWithResponse(t.Context())
	if err != nil || resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		t.Fatal("No LLM models available in the backend — TestAccLlmModelResource requires at least one model. Configure an LLM provider on the backend and re-run.")
	}

	return (*resp.JSON200)[0].ModelId
}

func TestAccLlmModelResource(t *testing.T) {
	modelID := testAccGetFirstModelID(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLlmModelResourceConfig(modelID, "1.00", "2.00"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("archestra_llm_model.test", "id"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "model_id", modelID),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_price_per_million_input", "1.00"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_price_per_million_output", "2.00"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "is_custom_price", "true"),
					resource.TestCheckResourceAttrSet("archestra_llm_model.test", "llm_provider"),
				),
			},
			{
				ResourceName:      "archestra_llm_model.test",
				ImportState:       true,
				ImportStateId:     modelID,
				ImportStateVerify: true,
			},
			{
				Config: testAccLlmModelResourceConfig(modelID, "3.00", "6.00"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_price_per_million_input", "3.00"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_price_per_million_output", "6.00"),
				),
			},
		},
	})
}

func TestAccLlmModelResourceIgnored(t *testing.T) {
	modelID := testAccGetFirstModelID(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLlmModelResourceIgnoredConfig(modelID, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_llm_model.test", "ignored", "true"),
				),
			},
			{
				Config: testAccLlmModelResourceIgnoredConfig(modelID, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_llm_model.test", "ignored", "false"),
				),
			},
		},
	})
}

func TestAccLlmModelResourceClearPricing(t *testing.T) {
	modelID := testAccGetFirstModelID(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with custom pricing
			{
				Config: testAccLlmModelResourceConfig(modelID, "5.00", "10.00"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_llm_model.test", "is_custom_price", "true"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_price_per_million_input", "5.00"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_price_per_million_output", "10.00"),
				),
			},
			// Update to remove custom pricing (don't set the fields)
			{
				Config: testAccLlmModelResourceNoPricingConfig(modelID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_llm_model.test", "is_custom_price", "false"),
				),
			},
		},
	})
}

func testAccLlmModelResourceNoPricingConfig(modelID string) string {
	return fmt.Sprintf(`
resource "archestra_llm_model" "test" {
  model_id = %[1]q
}
`, modelID)
}

func testAccLlmModelResourceConfig(modelID, inputPrice, outputPrice string) string {
	return fmt.Sprintf(`
resource "archestra_llm_model" "test" {
  model_id                        = %[1]q
  custom_price_per_million_input  = %[2]q
  custom_price_per_million_output = %[3]q
}
`, modelID, inputPrice, outputPrice)
}

func testAccLlmModelResourceIgnoredConfig(modelID string, ignored bool) string {
	return fmt.Sprintf(`
resource "archestra_llm_model" "test" {
  model_id = %[1]q
  ignored  = %[2]t
}
`, modelID, ignored)
}
