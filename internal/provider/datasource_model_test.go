package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccFirstModelWithKey returns a discovered model and the name of a key it
// is reached through, or empty strings when acceptance tests are off.
func testAccFirstModelWithKey(t *testing.T) (modelID, provider, keyName string) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		return "", "", ""
	}
	apiKey := os.Getenv("ARCHESTRA_API_KEY")
	c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", apiKey)
		return nil
	}))
	if err != nil {
		t.Fatalf("Unable to create client: %s", err)
	}
	resp, err := c.GetModelsWithApiKeysWithResponse(t.Context(), nil)
	if err != nil || resp.JSON200 == nil {
		t.Fatalf("Unable to list models: %v", err)
	}
	for _, model := range *resp.JSON200 {
		if len(model.ApiKeys) > 0 {
			return model.ModelId, string(model.Provider), model.ApiKeys[0].Name
		}
	}
	t.Fatal("No model with a provider key: seed a test provider key before running model acceptance tests.")
	return "", "", ""
}

// A model and the key it is reached through are found by name, and their IDs
// are the ones an agent pins.
func TestAccLlmModelAndModelProviderDataSources(t *testing.T) {
	modelID, provider, keyName := testAccFirstModelWithKey(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "archestra_model" "test" {
  model_id     = %[1]q
  llm_provider = %[2]q
}

# A key the model is reached through, found by name.
data "archestra_model_provider" "test" {
  name         = %[3]q
  llm_provider = data.archestra_model.test.llm_provider
}

resource "archestra_agent" "test" {
  name           = "tf-acc-model-data-sources"
  system_prompt  = "Test."
  model_id       = data.archestra_model.test.id
  llm_api_key_id = data.archestra_model_provider.test.id
}
`, modelID, provider, keyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.archestra_model.test", "model_id", modelID),
					resource.TestCheckResourceAttr("data.archestra_model_provider.test", "name", keyName),
					resource.TestCheckResourceAttrPair("archestra_agent.test", "model_id", "data.archestra_model.test", "id"),
					resource.TestCheckResourceAttrPair("archestra_agent.test", "llm_api_key_id", "data.archestra_model_provider.test", "id"),
					resource.TestCheckTypeSetElemAttrPair("data.archestra_model.test", "api_key_ids.*", "data.archestra_model_provider.test", "id"),
				),
			},
			{
				Config:      `data "archestra_model" "missing" { model_id = "tf-acc-no-such-model" }`,
				ExpectError: regexp.MustCompile(`Model Not Found`),
			},
			{
				Config:      `data "archestra_model_provider" "missing" { name = "tf-acc-no-such-key" }`,
				ExpectError: regexp.MustCompile(`Provider Key Not Found`),
			},
			{
				Config:      `data "archestra_model_provider" "empty" {}`,
				ExpectError: regexp.MustCompile(`Missing Lookup`),
			},
		},
	})
}
