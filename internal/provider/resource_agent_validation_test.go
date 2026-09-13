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
	rootType := schemaResponse.Schema.Type().TerraformType(ctx).(tftypes.Object)
	for _, scope := range []string{"org", "team"} {
		t.Run(scope, func(t *testing.T) {
			values := make(map[string]tftypes.Value)
			for name, typ := range rootType.AttributeTypes {
				values[name] = tftypes.NewValue(typ, nil)
			}
			values["scope"] = tftypes.NewValue(tftypes.String, scope)
			runtimeType := rootType.AttributeTypes["runtime"].(tftypes.Object)
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
			if scope == "org" && response.Diagnostics.HasError() {
				t.Fatalf("unknown conditional runtime must validate: %v", response.Diagnostics)
			}
			if scope == "team" {
				if len(response.Diagnostics) != 1 || response.Diagnostics[0].Detail() != `teams must be set when scope = "team"` {
					t.Fatalf("cross-field validation must still run: %v", response.Diagnostics)
				}
			}
		})
	}
}
