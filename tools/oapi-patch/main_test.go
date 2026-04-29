package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// runPatch is the test helper: parse JSON, run patch, return result + stats.
func runPatch(t *testing.T, in string) (map[string]any, *patchStats) {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(in), &doc); err != nil {
		t.Fatalf("parse input: %v", err)
	}
	stats := &patchStats{}
	out := patch(doc, stats)
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", out)
	}
	return m, stats
}

func TestPatch_EmbeddingDimensionsCollapsesToIntegerEnum(t *testing.T) {
	in := `{
		"nullable": true,
		"anyOf": [
			{"type": "number", "enum": [3072]},
			{"type": "number", "enum": [1536]},
			{"type": "number", "enum": [768]}
		]
	}`
	got, stats := runPatch(t, in)
	if stats.collapsed != 1 {
		t.Errorf("collapsed=%d want 1", stats.collapsed)
	}
	if got["type"] != "integer" {
		t.Errorf("type=%v want integer", got["type"])
	}
	if got["nullable"] != true {
		t.Errorf("nullable=%v want true", got["nullable"])
	}
	if _, hasAnyOf := got["anyOf"]; hasAnyOf {
		t.Errorf("anyOf should be removed, got %v", got["anyOf"])
	}
	enum, _ := got["enum"].([]any)
	if len(enum) != 3 {
		t.Fatalf("enum len=%d want 3", len(enum))
	}
	// String-sorted: 1536, 3072, 768
	want := []any{1536.0, 3072.0, 768.0}
	if !reflect.DeepEqual(enum, want) {
		t.Errorf("enum=%v want %v", enum, want)
	}
}

func TestPatch_StringConstArmsCollapseToStringEnum(t *testing.T) {
	in := `{
		"anyOf": [
			{"type": "string", "enum": ["jira"]},
			{"type": "string", "enum": ["confluence"]},
			{"type": "string", "enum": ["github"]}
		]
	}`
	got, stats := runPatch(t, in)
	if stats.collapsed != 1 {
		t.Errorf("collapsed=%d want 1", stats.collapsed)
	}
	if got["type"] != "string" {
		t.Errorf("type=%v want string", got["type"])
	}
	enum, _ := got["enum"].([]any)
	want := []any{"confluence", "github", "jira"}
	if !reflect.DeepEqual(enum, want) {
		t.Errorf("enum=%v want %v", enum, want)
	}
}

func TestPatch_MixedPrimitiveArmsBecomeFreeForm(t *testing.T) {
	in := `{
		"anyOf": [
			{"type": "string"},
			{"type": "number"},
			{"type": "boolean"}
		]
	}`
	got, stats := runPatch(t, in)
	if stats.freeFormed != 1 {
		t.Errorf("freeFormed=%d want 1", stats.freeFormed)
	}
	if _, hasAnyOf := got["anyOf"]; hasAnyOf {
		t.Errorf("anyOf should be removed")
	}
	if _, hasType := got["type"]; hasType {
		t.Errorf("type should be stripped to allow *interface{}, got %v", got["type"])
	}
}

func TestPatch_PrimitiveArrayMixBecomesFreeForm(t *testing.T) {
	// `string | array<string>` (e.g. `Anthropic system` field, but in a
	// non-excluded endpoint). oapi-codegen would emit a broken-union for
	// inline shapes; free-forming yields *interface{} which JSON-roundtrips.
	in := `{
		"anyOf": [
			{"type": "string"},
			{"type": "array", "items": {"type": "string"}}
		]
	}`
	got, stats := runPatch(t, in)
	if stats.freeFormed != 1 {
		t.Errorf("freeFormed=%d want 1", stats.freeFormed)
	}
	if _, hasAnyOf := got["anyOf"]; hasAnyOf {
		t.Errorf("anyOf should be removed")
	}
	if _, hasType := got["type"]; hasType {
		t.Errorf("type should be stripped to allow *interface{}, got %v", got["type"])
	}
}

func TestPatch_FullyMixedFourArmsBecomeFreeForm(t *testing.T) {
	// UserConfigFieldDefault-shape: string | number | boolean | array<string>.
	// Named version works (codegen handles), but if it ever appeared inline we
	// want free-form, not preserve.
	in := `{
		"anyOf": [
			{"type": "string"},
			{"type": "number"},
			{"type": "boolean"},
			{"type": "array", "items": {"type": "string"}}
		]
	}`
	_, stats := runPatch(t, in)
	if stats.freeFormed != 1 {
		t.Errorf("freeFormed=%d want 1", stats.freeFormed)
	}
}

func TestPatch_ObjectArmsPreserved(t *testing.T) {
	// Object-arm unions are left for oapi-codegen + the provider's
	// raw-body parse pattern; we don't want to flatten z.discriminatedUnion
	// schemas into permissive blobs and lose all the typed accessors.
	in := `{
		"type": "array",
		"items": {
			"anyOf": [
				{"type": "object", "properties": {"maxLength": {"type": "integer"}}, "required": ["maxLength"]},
				{"type": "object", "properties": {"hasTools": {"type": "boolean"}}, "required": ["hasTools"]}
			]
		}
	}`
	got, stats := runPatch(t, in)
	if stats.preserved != 1 {
		t.Errorf("preserved=%d want 1", stats.preserved)
	}
	items, _ := got["items"].(map[string]any)
	if _, hasAnyOf := items["anyOf"]; !hasAnyOf {
		t.Errorf("object-arm anyOf should be left untouched, got %v", items)
	}
}

func TestPatch_NamedRefUnionPreserved(t *testing.T) {
	in := `{
		"oneOf": [
			{"$ref": "#/components/schemas/Cat"},
			{"$ref": "#/components/schemas/Dog"}
		]
	}`
	got, stats := runPatch(t, in)
	if stats.preserved != 1 {
		t.Errorf("preserved=%d want 1", stats.preserved)
	}
	if _, hasOneOf := got["oneOf"]; !hasOneOf {
		t.Errorf("named-ref oneOf should be untouched")
	}
}

func TestPatch_DiscriminatorPreserved(t *testing.T) {
	in := `{
		"oneOf": [
			{"type": "object", "properties": {"kind": {"const": "a"}}},
			{"type": "object", "properties": {"kind": {"const": "b"}}}
		],
		"discriminator": {"propertyName": "kind"}
	}`
	got, stats := runPatch(t, in)
	if stats.preserved != 1 {
		t.Errorf("preserved=%d want 1", stats.preserved)
	}
	if _, hasOneOf := got["oneOf"]; !hasOneOf {
		t.Errorf("discriminator-tagged oneOf should be untouched")
	}
}

func TestPatch_SingleArmUnwraps(t *testing.T) {
	in := `{"anyOf": [{"type": "string", "enum": ["only"]}]}`
	got, stats := runPatch(t, in)
	if stats.unwrapped != 1 {
		t.Errorf("unwrapped=%d want 1", stats.unwrapped)
	}
	if got["type"] != "string" {
		t.Errorf("expected unwrapped to type=string, got %v", got["type"])
	}
}

func TestPatch_PreservesNullableAndDescriptionOnCollapse(t *testing.T) {
	in := `{
		"nullable": true,
		"description": "embedding column dimensions",
		"anyOf": [
			{"type": "number", "enum": [768]},
			{"type": "number", "enum": [1536]}
		]
	}`
	got, _ := runPatch(t, in)
	if got["nullable"] != true {
		t.Error("nullable lost")
	}
	if got["description"] != "embedding column dimensions" {
		t.Error("description lost")
	}
}

func TestPatch_Idempotent(t *testing.T) {
	in := `{
		"paths": {
			"/x": {
				"get": {
					"responses": {
						"200": {
							"content": {
								"application/json": {
									"schema": {
										"properties": {
											"dim": {"anyOf": [{"type":"number","enum":[768]},{"type":"number","enum":[1536]}]},
											"def": {"anyOf": [{"type":"string"},{"type":"number"}]}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`
	once := mustEncode(t, runPatchOnce(t, in))
	twice := mustEncode(t, runPatchOnce(t, once))
	if once != twice {
		t.Errorf("non-idempotent\nfirst:  %s\nsecond: %s", once, twice)
	}
}

func runPatchOnce(t *testing.T, s string) string {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	patched := patch(doc, &patchStats{})
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(patched); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.String()
}

func mustEncode(t *testing.T, v any) string {
	t.Helper()
	switch s := v.(type) {
	case string:
		return s
	default:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			t.Fatalf("encode: %v", err)
		}
		return buf.String()
	}
}
