package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRemoveOnConfigNullList(t *testing.T) {
	t.Parallel()

	listOf := func(elems ...string) types.List {
		v, diags := types.ListValueFrom(t.Context(), types.StringType, elems)
		if diags.HasError() {
			t.Fatalf("ListValueFrom: %v", diags)
		}
		return v
	}

	cases := []struct {
		name     string
		config   types.List
		state    types.List
		plan     types.List
		wantPlan types.List
	}{
		{
			name:     "config null, state has value -> plan null",
			config:   types.ListNull(types.StringType),
			state:    listOf("a", "b"),
			plan:     types.ListUnknown(types.StringType),
			wantPlan: types.ListNull(types.StringType),
		},
		{
			name:     "config null, state null, plan unknown (first apply) -> plan null",
			config:   types.ListNull(types.StringType),
			state:    types.ListNull(types.StringType),
			plan:     types.ListUnknown(types.StringType),
			wantPlan: types.ListNull(types.StringType),
		},
		{
			name:     "config null, state unknown, plan unknown (refresh) -> plan null",
			config:   types.ListNull(types.StringType),
			state:    types.ListUnknown(types.StringType),
			plan:     types.ListUnknown(types.StringType),
			wantPlan: types.ListNull(types.StringType),
		},
		{
			name:     "config has value -> no-op",
			config:   listOf("x"),
			state:    listOf("a", "b"),
			plan:     listOf("x"),
			wantPlan: listOf("x"),
		},
		{
			name:     "config null, plan concrete from prior modifier -> cleared",
			config:   types.ListNull(types.StringType),
			state:    listOf("a"),
			plan:     listOf("a"),
			wantPlan: types.ListNull(types.StringType),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.ListRequest{
				ConfigValue: tc.config,
				StateValue:  tc.state,
				PlanValue:   tc.plan,
			}
			resp := &planmodifier.ListResponse{PlanValue: tc.plan}

			RemoveOnConfigNullList().PlanModifyList(t.Context(), req, resp)

			if !resp.PlanValue.Equal(tc.wantPlan) {
				t.Errorf("plan value:\n  got:  %s\n  want: %s", resp.PlanValue, tc.wantPlan)
			}
		})
	}
}
