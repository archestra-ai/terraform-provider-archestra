package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// objType is a small helper to build a tftypes.Object type from a flat
// attribute spec, reducing boilerplate in tests. Returns tftypes.Object
// (concrete type) rather than tftypes.Type (interface) so call sites can
// access AttributeTypes directly without a type assertion.
func objType(attrs map[string]tftypes.Type) tftypes.Object {
	return tftypes.Object{AttributeTypes: attrs}
}

// objVal is a small helper to build a tftypes.Object value from a literal
// map. Pass nil for null attributes.
func objVal(attrs map[string]tftypes.Type, fields map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(objType(attrs), fields)
}

func TestMergePatch_Scalar(t *testing.T) {
	t.Parallel()

	objT := objType(map[string]tftypes.Type{
		"name":  tftypes.String,
		"count": tftypes.Number,
		"on":    tftypes.Bool,
	})
	spec := []AttrSpec{
		{TFName: "name", JSONName: "name", Kind: Scalar},
		{TFName: "count", JSONName: "count", Kind: Scalar},
		{TFName: "on", JSONName: "on", Kind: Scalar},
	}

	tests := []struct {
		name  string
		plan  tftypes.Value
		prior tftypes.Value
		want  map[string]any
	}{
		{
			name: "all equal — empty patch",
			plan: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, "x"),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, true),
			}),
			prior: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, "x"),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, true),
			}),
			want: map[string]any{},
		},
		{
			name: "scalar changed — emits new value",
			plan: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, "y"),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, true),
			}),
			prior: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, "x"),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, true),
			}),
			want: map[string]any{"name": "y"},
		},
		{
			name: "value → null — emits JSON null",
			plan: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, nil),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, true),
			}),
			prior: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, "x"),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, true),
			}),
			want: map[string]any{"name": nil},
		},
		{
			name: "null → value — emits new value",
			plan: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, "x"),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, true),
			}),
			prior: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, nil),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, true),
			}),
			want: map[string]any{"name": "x"},
		},
		{
			name: "null → null — omitted",
			plan: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, nil),
				"count": tftypes.NewValue(tftypes.Number, nil),
				"on":    tftypes.NewValue(tftypes.Bool, nil),
			}),
			prior: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, nil),
				"count": tftypes.NewValue(tftypes.Number, nil),
				"on":    tftypes.NewValue(tftypes.Bool, nil),
			}),
			want: map[string]any{},
		},
		{
			name: "Create — null prior emits every non-null plan attr",
			plan: objVal(objT.AttributeTypes, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, "x"),
				"count": tftypes.NewValue(tftypes.Number, 5),
				"on":    tftypes.NewValue(tftypes.Bool, nil),
			}),
			prior: tftypes.NewValue(objT, nil),
			want:  map[string]any{"name": "x", "count": float64(5)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			got := MergePatch(t.Context(), tt.plan, tt.prior, spec, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diags: %v", diags)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergePatch_List(t *testing.T) {
	t.Parallel()

	listType := tftypes.List{ElementType: tftypes.String}
	objT := objType(map[string]tftypes.Type{"tags": listType})
	spec := []AttrSpec{{TFName: "tags", JSONName: "tags", Kind: List}}

	cases := []struct {
		name  string
		plan  []tftypes.Value
		prior []tftypes.Value
		want  map[string]any
	}{
		{
			name:  "equal lists omitted",
			plan:  []tftypes.Value{tftypes.NewValue(tftypes.String, "a"), tftypes.NewValue(tftypes.String, "b")},
			prior: []tftypes.Value{tftypes.NewValue(tftypes.String, "a"), tftypes.NewValue(tftypes.String, "b")},
			want:  map[string]any{},
		},
		{
			name:  "added element — emits whole new list",
			plan:  []tftypes.Value{tftypes.NewValue(tftypes.String, "a"), tftypes.NewValue(tftypes.String, "b")},
			prior: []tftypes.Value{tftypes.NewValue(tftypes.String, "a")},
			want:  map[string]any{"tags": []any{"a", "b"}},
		},
		{
			name:  "removed element — emits whole new list",
			plan:  []tftypes.Value{tftypes.NewValue(tftypes.String, "a")},
			prior: []tftypes.Value{tftypes.NewValue(tftypes.String, "a"), tftypes.NewValue(tftypes.String, "b")},
			want:  map[string]any{"tags": []any{"a"}},
		},
		{
			name:  "reordered — emits whole new list (order matters for List)",
			plan:  []tftypes.Value{tftypes.NewValue(tftypes.String, "b"), tftypes.NewValue(tftypes.String, "a")},
			prior: []tftypes.Value{tftypes.NewValue(tftypes.String, "a"), tftypes.NewValue(tftypes.String, "b")},
			want:  map[string]any{"tags": []any{"b", "a"}},
		},
		{
			name:  "value → empty — emits empty list (clears entries)",
			plan:  []tftypes.Value{},
			prior: []tftypes.Value{tftypes.NewValue(tftypes.String, "a")},
			want:  map[string]any{"tags": []any{}},
		},
		{
			name:  "empty → empty — omitted",
			plan:  []tftypes.Value{},
			prior: []tftypes.Value{},
			want:  map[string]any{},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			plan := objVal(map[string]tftypes.Type{"tags": listType},
				map[string]tftypes.Value{"tags": tftypes.NewValue(listType, tt.plan)})
			prior := objVal(map[string]tftypes.Type{"tags": listType},
				map[string]tftypes.Value{"tags": tftypes.NewValue(listType, tt.prior)})

			var diags diag.Diagnostics
			got := MergePatch(t.Context(), plan, prior, spec, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diags: %v", diags)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			_ = objT
		})
	}
}

func TestMergePatch_AtomicObject(t *testing.T) {
	t.Parallel()

	innerT := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"client_id":     tftypes.String,
		"client_secret": tftypes.String,
		"pkce":          tftypes.Bool,
	}}
	outerT := map[string]tftypes.Type{"oidc_config": innerT}
	spec := []AttrSpec{{
		TFName:   "oidc_config",
		JSONName: "oidcConfig",
		Kind:     AtomicObject,
		Children: []AttrSpec{
			{TFName: "client_id", JSONName: "clientId", Kind: Scalar},
			{TFName: "client_secret", JSONName: "clientSecret", Kind: Scalar, Sensitive: true},
			{TFName: "pkce", JSONName: "pkce", Kind: Scalar},
		},
	}}

	mkInner := func(id, secret string, pkce bool) tftypes.Value {
		return tftypes.NewValue(innerT, map[string]tftypes.Value{
			"client_id":     tftypes.NewValue(tftypes.String, id),
			"client_secret": tftypes.NewValue(tftypes.String, secret),
			"pkce":          tftypes.NewValue(tftypes.Bool, pkce),
		})
	}

	t.Run("equal — omitted", func(t *testing.T) {
		plan := objVal(outerT, map[string]tftypes.Value{"oidc_config": mkInner("a", "s", true)})
		prior := objVal(outerT, map[string]tftypes.Value{"oidc_config": mkInner("a", "s", true)})

		var diags diag.Diagnostics
		got := MergePatch(t.Context(), plan, prior, spec, &diags)
		if diags.HasError() {
			t.Fatalf("unexpected diags: %v", diags)
		}
		if !reflect.DeepEqual(got, map[string]any{}) {
			t.Errorf("expected empty patch, got %v", got)
		}
	})

	t.Run("sub-field changed — emits whole object with re-keyed JSON names", func(t *testing.T) {
		plan := objVal(outerT, map[string]tftypes.Value{"oidc_config": mkInner("a", "new", true)})
		prior := objVal(outerT, map[string]tftypes.Value{"oidc_config": mkInner("a", "old", true)})

		var diags diag.Diagnostics
		got := MergePatch(t.Context(), plan, prior, spec, &diags)
		if diags.HasError() {
			t.Fatalf("unexpected diags: %v", diags)
		}
		want := map[string]any{
			"oidcConfig": map[string]any{
				"clientId":     "a",
				"clientSecret": "new",
				"pkce":         true,
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("value → null — emits JSON null (clears the column)", func(t *testing.T) {
		plan := objVal(outerT, map[string]tftypes.Value{"oidc_config": tftypes.NewValue(innerT, nil)})
		prior := objVal(outerT, map[string]tftypes.Value{"oidc_config": mkInner("a", "s", true)})

		var diags diag.Diagnostics
		got := MergePatch(t.Context(), plan, prior, spec, &diags)
		if diags.HasError() {
			t.Fatalf("unexpected diags: %v", diags)
		}
		if !reflect.DeepEqual(got, map[string]any{"oidcConfig": nil}) {
			t.Errorf("expected oidcConfig=nil, got %v", got)
		}
	})
}

func TestMergePatch_RecursiveObject(t *testing.T) {
	t.Parallel()

	innerT := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"a": tftypes.String,
		"b": tftypes.String,
	}}
	outerT := map[string]tftypes.Type{"meta": innerT}
	spec := []AttrSpec{{
		TFName:   "meta",
		JSONName: "meta",
		Kind:     RecursiveObject,
		Children: []AttrSpec{
			{TFName: "a", JSONName: "a", Kind: Scalar},
			{TFName: "b", JSONName: "b", Kind: Scalar},
		},
	}}

	mk := func(a, b string) tftypes.Value {
		return tftypes.NewValue(innerT, map[string]tftypes.Value{
			"a": tftypes.NewValue(tftypes.String, a),
			"b": tftypes.NewValue(tftypes.String, b),
		})
	}

	plan := objVal(outerT, map[string]tftypes.Value{"meta": mk("changed", "same")})
	prior := objVal(outerT, map[string]tftypes.Value{"meta": mk("orig", "same")})

	var diags diag.Diagnostics
	got := MergePatch(t.Context(), plan, prior, spec, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}

	// Recursive: only the changed sub-field is in the sub-patch.
	want := map[string]any{"meta": map[string]any{"a": "changed"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMergePatch_EncoderApplied(t *testing.T) {
	t.Parallel()

	objT := map[string]tftypes.Type{"id": tftypes.String}
	spec := []AttrSpec{{
		TFName:   "id",
		JSONName: "id",
		Kind:     Scalar,
		Encoder: func(v any) any {
			s, _ := v.(string)
			return "encoded:" + s
		},
	}}

	plan := objVal(objT, map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "x")})
	prior := tftypes.NewValue(objType(objT), nil)

	var diags diag.Diagnostics
	got := MergePatch(t.Context(), plan, prior, spec, &diags)
	if got["id"] != "encoded:x" {
		t.Errorf("expected encoder applied, got %v", got["id"])
	}
}

func TestMergePatch_UnknownPlanOmitted(t *testing.T) {
	t.Parallel()

	objT := map[string]tftypes.Type{"id": tftypes.String}
	spec := []AttrSpec{{TFName: "id", JSONName: "id", Kind: Scalar}}

	plan := objVal(objT, map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, tftypes.UnknownValue)})
	prior := objVal(objT, map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "old")})

	var diags diag.Diagnostics
	got := MergePatch(t.Context(), plan, prior, spec, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if !reflect.DeepEqual(got, map[string]any{}) {
		t.Errorf("unknown plan should be omitted; got %v", got)
	}
}

func TestMaskSensitive(t *testing.T) {
	t.Parallel()

	spec := []AttrSpec{
		{TFName: "name", JSONName: "name", Kind: Scalar},
		{TFName: "client_secret", JSONName: "clientSecret", Kind: Scalar, Sensitive: true},
		{
			TFName:   "oidc_config",
			JSONName: "oidcConfig",
			Kind:     AtomicObject,
			Children: []AttrSpec{
				{TFName: "client_id", JSONName: "clientId", Kind: Scalar},
				{TFName: "client_secret", JSONName: "clientSecret", Kind: Scalar, Sensitive: true},
			},
		},
	}

	patch := map[string]any{
		"name":         "foo",
		"clientSecret": "topsecret",
		"oidcConfig": map[string]any{
			"clientId":     "abc",
			"clientSecret": "morsel",
		},
	}

	got := maskSensitive(patch, spec)
	want := map[string]any{
		"name":         "foo",
		"clientSecret": "***",
		"oidcConfig": map[string]any{
			"clientId":     "abc",
			"clientSecret": "***",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
