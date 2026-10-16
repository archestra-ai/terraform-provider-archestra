package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestAgentValidationAllowsUnknownRuntimeSettings(t *testing.T) {
	ctx := context.Background()
	r := &AgentResource{}
	var schemaResponse resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	rootType, ok := schemaResponse.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatal("expected object resource schema")
	}
	t.Run("unknown runtime", func(t *testing.T) {
		values := make(map[string]tftypes.Value)
		for name, typ := range rootType.AttributeTypes {
			values[name] = tftypes.NewValue(typ, nil)
		}
		runtimeType, ok := rootType.AttributeTypes["runtime"].(tftypes.Object)
		if !ok {
			t.Fatal("expected object runtime schema")
		}
		runtimeValues := make(map[string]tftypes.Value)
		for name, typ := range runtimeType.AttributeTypes {
			runtimeValues[name] = tftypes.NewValue(typ, nil)
		}
		runtimeValues["claude_code"] = tftypes.NewValue(runtimeType.AttributeTypes["claude_code"], tftypes.UnknownValue)
		values["runtime"] = tftypes.NewValue(runtimeType, runtimeValues)
		var response resource.ValidateConfigResponse
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{
			Schema: schemaResponse.Schema,
			Raw:    tftypes.NewValue(rootType, values),
		}}, &response)
		if response.Diagnostics.HasError() {
			t.Fatalf("unknown conditional runtime must validate: %v", response.Diagnostics)
		}
	})
}
