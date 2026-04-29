package provider

import (
	"testing"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// TestReconcileDefaultPolicyTools pins the drift-detection contract used
// by archestra_tool_invocation_policy_default and
// archestra_trusted_data_policy_default. The function must keep tools
// whose backend default still matches state.action and drop the rest —
// out-of-band action changes, deleted policy rows, and deleted tools all
// fall out of state and re-surface as `+ tool_id` plan diffs that the
// next apply re-asserts.
func TestReconcileDefaultPolicyTools(t *testing.T) {
	a := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	b := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	c := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	d := uuid.MustParse("00000000-0000-0000-0000-000000000004")

	tests := []struct {
		name        string
		stateTools  []openapi_types.UUID
		stateAction string
		defaults    map[openapi_types.UUID]string
		want        []openapi_types.UUID
	}{
		{
			name:        "all match — full set kept",
			stateTools:  []openapi_types.UUID{a, b, c},
			stateAction: "block_always",
			defaults:    map[openapi_types.UUID]string{a: "block_always", b: "block_always", c: "block_always"},
			want:        []openapi_types.UUID{a, b, c},
		},
		{
			name:        "one tool drifted to different action — dropped",
			stateTools:  []openapi_types.UUID{a, b, c},
			stateAction: "block_always",
			defaults:    map[openapi_types.UUID]string{a: "block_always", b: "require_approval", c: "block_always"},
			want:        []openapi_types.UUID{a, c},
		},
		{
			name:        "tool missing from backend (no default row) — dropped",
			stateTools:  []openapi_types.UUID{a, b, c},
			stateAction: "block_always",
			defaults:    map[openapi_types.UUID]string{a: "block_always", c: "block_always"},
			want:        []openapi_types.UUID{a, c},
		},
		{
			name:        "all tools drifted away — empty set, caller removes resource",
			stateTools:  []openapi_types.UUID{a, b},
			stateAction: "block_always",
			defaults:    map[openapi_types.UUID]string{a: "require_approval", b: "require_approval"},
			want:        []openapi_types.UUID{},
		},
		{
			name:        "extra defaults for unmanaged tools — ignored",
			stateTools:  []openapi_types.UUID{a},
			stateAction: "block_always",
			defaults:    map[openapi_types.UUID]string{a: "block_always", d: "block_always"},
			want:        []openapi_types.UUID{a},
		},
		{
			name:        "empty state — empty result",
			stateTools:  []openapi_types.UUID{},
			stateAction: "block_always",
			defaults:    map[openapi_types.UUID]string{a: "block_always"},
			want:        []openapi_types.UUID{},
		},
		{
			name:        "preserves stateTools order",
			stateTools:  []openapi_types.UUID{c, a, b},
			stateAction: "block_always",
			defaults:    map[openapi_types.UUID]string{a: "block_always", b: "block_always", c: "block_always"},
			want:        []openapi_types.UUID{c, a, b},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reconcileDefaultPolicyTools(tc.stateTools, tc.stateAction, tc.defaults)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %d %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("position %d: got %s, want %s", i, got[i], tc.want[i])
				}
			}
		})
	}
}
