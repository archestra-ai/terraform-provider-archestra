package provider

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestLLMProviderNamesCoverCurrentAPI(t *testing.T) {
	source, err := os.ReadFile("../client/archestra_client.go")
	if err != nil {
		t.Fatal(err)
	}
	names := regexp.MustCompile(`CreateLlmProviderApiKeyJSONBodyProvider\w+\s+CreateLlmProviderApiKeyJSONBodyProvider = "([^"]+)"`).FindAllSubmatch(source, -1)
	if len(names) == 0 {
		t.Fatal("no current API provider enum values found")
	}
	var response resource.SchemaResponse
	(&LLMProviderApiKeyResource{}).Schema(t.Context(), resource.SchemaRequest{}, &response)
	attribute, ok := response.Schema.Attributes["llm_provider"].(schema.StringAttribute)
	if !ok {
		t.Fatal("llm_provider must have a string schema")
	}
	for _, match := range names {
		name := string(match[1])
		t.Run(name, func(t *testing.T) {
			for _, check := range attribute.Validators {
				var result validator.StringResponse
				check.ValidateString(t.Context(), validator.StringRequest{Path: path.Root("llm_provider"), ConfigValue: types.StringValue(name)}, &result)
				if result.Diagnostics.HasError() {
					t.Fatalf("current API provider %s is rejected by the schema", name)
				}
			}
		})
	}
}
