package provider

import (
	"context"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestAccCurrentModelOverrides(t *testing.T) {
	baseURL := os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL")
	keyOnly := fmt.Sprintf(`resource "archestra_llm_provider_api_key" "model_fixture" {
name="tf-native-model-fixture"
llm_provider="ollama-native"
base_url=%q
}`, baseURL)
	var modelUUID string
	apiClient := func() *client.ClientWithResponses {
		c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	config := func(extra string) string {
		return keyOnly + fmt.Sprintf(`
 resource "archestra_llm_model" "test" {
 model_id="llama3"
 llm_provider="ollama-native"
 depends_on=[archestra_llm_provider_api_key.model_fixture]
 %s
 }
 `, extra)
	}
	initial := config(`custom_context_length=2048
 custom_output_length=256
 custom_price_per_million_cache_read="0.10"
 custom_price_per_million_cache_write="0.20"
 configured_parameters=jsonencode({temperature=0.6,num_ctx=1024})
 labels=[{key="stage",value="initial"}]`)
	updated := config(`custom_context_length=4096
 custom_output_length=512
 custom_price_per_million_cache_read="0.30"
 custom_price_per_million_cache_write="0.40"
 configured_parameters=jsonencode({temperature=0.8,num_ctx=2048})
 labels=[{key="stage",value="updated"}]`)
	cleared := config(`labels=[]`)
	withModalities := strings.Replace(updated, "depends_on=", "input_modalities=[\"text\",\"image\"]\noutput_modalities=[\"text\"]\ndepends_on=", 1)
	resource.Test(t, resource.TestCase{PreCheck: func() {
		testAccPreCheck(t)
		if baseURL == "" {
			t.Fatal("ARCHESTRA_TEST_LLM_BASE_URL must point to the isolated Ollama mock")
		}
	}, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_context_length", "2048"), resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_output_length", "256"), resource.TestCheckResourceAttr("archestra_llm_model.test", "labels.0.value", "initial"), func(state *terraform.State) error {
			modelUUID = state.RootModule().Resources["archestra_llm_model.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_llm_model.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccModelUUID},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_context_length", "4096"), resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_price_per_million_cache_read", "0.30"))},
		{Config: updated, PlanOnly: true},
		{Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
			id, err := uuid.Parse(modelUUID)
			if err != nil {
				t.Fatal(err)
			}
			result, err := apiClient().UpdateModelWithBodyWithResponse(t.Context(), id, "application/json", strings.NewReader(`{"customContextLength":3072}`))
			if err != nil || result.JSON200 == nil {
				t.Fatal("unable to create isolated model drift", err)
			}
		}},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_llm_model.test", "custom_context_length", "4096")},
		{Config: withModalities, Check: resource.TestCheckResourceAttr("archestra_llm_model.test", "input_modalities.#", "2")},
		{Config: updated, Check: resource.TestCheckNoResourceAttr("archestra_llm_model.test", "input_modalities")},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckNoResourceAttr("archestra_llm_model.test", "custom_context_length"), resource.TestCheckNoResourceAttr("archestra_llm_model.test", "configured_parameters"))},
		{Config: cleared, PlanOnly: true},
		{Config: updated},
		{Config: keyOnly, Check: func(_ *terraform.State) error {
			result, err := apiClient().GetModelsWithApiKeysWithResponse(t.Context(), nil)
			if err != nil {
				return err
			}
			if result.JSON200 == nil {
				return fmt.Errorf("model reset verification returned %d", result.StatusCode())
			}
			for _, model := range *result.JSON200 {
				if model.Id.String() == modelUUID {
					if model.CustomContextLength != nil || model.CustomOutputLength != nil || model.ConfiguredParameters != nil || model.CustomPricePerMillionCacheRead != nil || model.CustomPricePerMillionCacheWrite != nil || model.Ignored || len(model.Labels) != 0 {
						return fmt.Errorf("destroy left model overrides behind")
					}
					return nil
				}
			}
			return fmt.Errorf("destroy deleted or lost the adopted model instead of resetting it")
		}},
	}})
}

func testAccModelUUID(state *terraform.State) (string, error) {
	return state.RootModule().Resources["archestra_llm_model.test"].Primary.ID, nil
}
