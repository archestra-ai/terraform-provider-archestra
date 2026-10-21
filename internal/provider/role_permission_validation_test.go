package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRoleRejectsUnsupportedOrganizationPermissions(t *testing.T) {
	for _, tc := range []struct {
		resource, action string
		wantError        bool
	}{
		{"agent", "update", true}, {"agent", "delete", false}, {"member", "impersonate", false}, {"chat", "full-view", false}, {"retired-resource", "read", true}, {"agent", "bogus-action", true},
	} {
		t.Run(tc.resource+"/"+tc.action, func(t *testing.T) {
			r := RoleResource{}
			ctx := t.Context()
			var schema resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schema)
			actions := types.ListValueMust(types.StringType, []attr.Value{types.StringValue(tc.action)})
			data := RoleResourceModel{Name: types.StringValue("fixture"), Permission: types.MapValueMust(types.ListType{ElemType: types.StringType}, map[string]attr.Value{tc.resource: actions})}
			state := tfsdk.State{Schema: schema.Schema}
			if ds := state.Set(ctx, &data); ds.HasError() {
				t.Fatal(ds)
			}
			var response resource.ValidateConfigResponse
			r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: state.Raw}}, &response)
			if response.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("diagnostics=%v", response.Diagnostics)
			}
		})
	}
}
