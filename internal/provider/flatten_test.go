package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenStringList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		api       []string
		wantNull  bool
		wantElems int
	}{
		{name: "nil → null", api: nil, wantNull: true},
		{name: "empty slice → typed empty list, not null", api: []string{}, wantNull: false, wantElems: 0},
		{name: "non-empty", api: []string{"a", "b"}, wantNull: false, wantElems: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			got := FlattenStringList(t.Context(), tt.api, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diags: %v", diags)
			}
			if got.IsNull() != tt.wantNull {
				t.Errorf("IsNull: got %v, want %v", got.IsNull(), tt.wantNull)
			}
			if !tt.wantNull && len(got.Elements()) != tt.wantElems {
				t.Errorf("element count: got %d, want %d", len(got.Elements()), tt.wantElems)
			}
			if got.ElementType(t.Context()) != types.StringType {
				t.Errorf("element type: got %v, want StringType", got.ElementType(t.Context()))
			}
		})
	}
}

func TestFlattenStringSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		api       []string
		wantNull  bool
		wantElems int
	}{
		{name: "nil → null", api: nil, wantNull: true},
		{name: "empty → typed empty set", api: []string{}, wantNull: false, wantElems: 0},
		{name: "non-empty", api: []string{"a", "b", "c"}, wantNull: false, wantElems: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			got := FlattenStringSet(t.Context(), tt.api, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diags: %v", diags)
			}
			if got.IsNull() != tt.wantNull {
				t.Errorf("IsNull: got %v, want %v", got.IsNull(), tt.wantNull)
			}
			if !tt.wantNull && len(got.Elements()) != tt.wantElems {
				t.Errorf("element count: got %d, want %d", len(got.Elements()), tt.wantElems)
			}
		})
	}
}

func TestFlattenStringMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		api       map[string]string
		wantNull  bool
		wantElems int
	}{
		{name: "nil → null", api: nil, wantNull: true},
		{name: "empty → typed empty map", api: map[string]string{}, wantNull: false, wantElems: 0},
		{name: "non-empty", api: map[string]string{"a": "1", "b": "2"}, wantNull: false, wantElems: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			got := FlattenStringMap(t.Context(), tt.api, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diags: %v", diags)
			}
			if got.IsNull() != tt.wantNull {
				t.Errorf("IsNull: got %v, want %v", got.IsNull(), tt.wantNull)
			}
			if !tt.wantNull && len(got.Elements()) != tt.wantElems {
				t.Errorf("element count: got %d, want %d", len(got.Elements()), tt.wantElems)
			}
		})
	}
}

func TestPreserveOnNil(t *testing.T) {
	t.Parallel()

	encode := types.StringValue

	t.Run("api nil → keep prior", func(t *testing.T) {
		prior := types.StringValue("kept")
		got := PreserveOnNil[string](nil, prior, encode)
		if got.ValueString() != "kept" {
			t.Errorf("expected prior preserved, got %q", got.ValueString())
		}
	})

	t.Run("api set → use api value", func(t *testing.T) {
		v := "fresh"
		prior := types.StringValue("stale")
		got := PreserveOnNil(&v, prior, encode)
		if got.ValueString() != "fresh" {
			t.Errorf("expected api value used, got %q", got.ValueString())
		}
	})

	t.Run("api nil + prior null → returns null", func(t *testing.T) {
		prior := types.StringNull()
		got := PreserveOnNil[string](nil, prior, encode)
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})
}
