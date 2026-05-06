package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type apiLabel = struct {
	Key     string              `json:"key"`
	KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
	Value   string              `json:"value"`
	ValueId *openapi_types.UUID `json:"valueId,omitempty"`
}

func TestFlattenAgentLabels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		prior     []AgentLabelModel
		api       []apiLabel
		wantNil   bool
		wantEmpty bool
		wantLen   int
	}{
		{
			name:    "null prior + empty api -> null state",
			prior:   nil,
			api:     nil,
			wantNil: true,
		},
		{
			// Without this case the user's `labels = []` HCL produces a
			// permanent null↔[] plan diff after every refresh.
			name:      "explicit empty prior + empty api -> empty state",
			prior:     []AgentLabelModel{},
			api:       []apiLabel{},
			wantEmpty: true,
		},
		{
			name:    "null prior + populated api -> populated state",
			prior:   nil,
			api:     []apiLabel{{Key: "team", Value: "platform"}},
			wantLen: 1,
		},
		{
			name:    "populated prior + populated api -> populated state",
			prior:   []AgentLabelModel{{Key: types.StringValue("old"), Value: types.StringValue("v")}},
			api:     []apiLabel{{Key: "team", Value: "platform"}, {Key: "env", Value: "prod"}},
			wantLen: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := flattenAgentLabels(tc.prior, tc.api)
			switch {
			case tc.wantNil:
				if got != nil {
					t.Fatalf("expected nil, got %v (len=%d)", got, len(got))
				}
			case tc.wantEmpty:
				if got == nil {
					t.Fatalf("expected empty slice, got nil")
				}
				if len(got) != 0 {
					t.Fatalf("expected empty slice, got len=%d", len(got))
				}
			default:
				if len(got) != tc.wantLen {
					t.Fatalf("expected len=%d, got len=%d", tc.wantLen, len(got))
				}
				for i, l := range got {
					if l.Key.ValueString() != tc.api[i].Key || l.Value.ValueString() != tc.api[i].Value {
						t.Fatalf("entry %d: got %s=%s, want %s=%s", i,
							l.Key.ValueString(), l.Value.ValueString(),
							tc.api[i].Key, tc.api[i].Value)
					}
				}
			}
		})
	}
}
