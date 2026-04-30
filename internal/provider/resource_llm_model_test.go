package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// testAccGetFirstModelID discovers the first available LLM model from the backend.
// Returns the model_id string. Acceptance tests gated by TF_ACC: when TF_ACC is
// unset, returns early so resource.Test handles the skip; when TF_ACC is set,
// fails loud (t.Fatal) on any setup defect — never silently skips.
func testAccGetFirstModelID(t *testing.T) string {
	t.Helper()

	if os.Getenv("TF_ACC") == "" {
		return ""
	}

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
	if err != nil {
		t.Fatalf("GetModelsWithApiKeys failed: %s — likely a generated-client/spec mismatch; regenerate via `make codegen-api-client` after backend bumps", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("GetModelsWithApiKeys returned %d: %s", resp.StatusCode(), string(resp.Body))
	}
	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		t.Skip("skipping: no LLM models available in the backend — configure an LLM provider with at least one model on the backend to exercise these tests. Backend-state-gated; CI without a seeded provider is the intended default.")
		return ""
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

// TestAccLlmModelResource_ModalitiesRemoveCycle pins RemoveOnConfigNullList
// on the modality fields. Per backend models/model.ts:402-407, sending null
// stores null on the row (cleared); a later periodic provider sync may
// repopulate via COALESCE, but the immediate post-apply state is null.
func TestAccLlmModelResource_ModalitiesRemoveCycle(t *testing.T) {
	modelID := testAccGetFirstModelID(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLlmModelResourceConfigWithModalities(modelID, []string{"text"}, []string{"text"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_llm_model.test", "input_modalities.#", "1"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "input_modalities.0", "text"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "output_modalities.#", "1"),
					resource.TestCheckResourceAttr("archestra_llm_model.test", "output_modalities.0", "text"),
				),
			},
			{
				Config: testAccLlmModelResourceNoPricingConfig(modelID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_llm_model.test", tfjsonpath.New("input_modalities"), knownvalue.Null()),
					statecheck.ExpectKnownValue("archestra_llm_model.test", tfjsonpath.New("output_modalities"), knownvalue.Null()),
				},
			},
		},
	})
}

func testAccLlmModelResourceConfigWithModalities(modelID string, in, out []string) string {
	quoted := func(xs []string) string {
		s := ""
		for i, x := range xs {
			if i > 0 {
				s += ", "
			}
			s += fmt.Sprintf("%q", x)
		}
		return s
	}
	return fmt.Sprintf(`
resource "archestra_llm_model" "test" {
  model_id          = %[1]q
  input_modalities  = [%[2]s]
  output_modalities = [%[3]s]
}
`, modelID, quoted(in), quoted(out))
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
