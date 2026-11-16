package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// An agent that declares `knowledge_base_ids = []` or `connector_ids = []`
// must keep the empty list after apply; the API reports no values either way.
func TestStringListFromAPIKeepsExplicitEmptyList(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		prior types.List
		api   []string
		want  types.List
	}{
		"explicit empty list stays empty": {types.ListValueMust(types.StringType, []attr.Value{}), nil, types.ListValueMust(types.StringType, []attr.Value{})},
		"omitted stays null":              {types.ListNull(types.StringType), nil, types.ListNull(types.StringType)},
		"unknown resolves to null":        {types.ListUnknown(types.StringType), nil, types.ListNull(types.StringType)},
		"cleared remotely becomes null":   {types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a")}), nil, types.ListNull(types.StringType)},
		"api values win":                  {types.ListValueMust(types.StringType, []attr.Value{}), []string{"a"}, types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a")})},
	} {
		t.Run(name, func(t *testing.T) {
			var diags diag.Diagnostics
			got := tc.prior
			stringListFromAPI(ctx, &got, tc.api, &diags)
			if diags.HasError() || !got.Equal(tc.want) {
				t.Fatalf("got %s (diagnostics %v), want %s", got, diags, tc.want)
			}
		})
	}
}
