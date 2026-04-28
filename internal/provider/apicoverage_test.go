package provider

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// resourceWithAPIShape is implemented by any resource that wants the
// API↔schema coverage check. The shape returned should be a zero-value of
// the generated client's `Get<Op>Response` struct (the one with `JSON200
// *struct{...}`); the test reflects over JSON200's anonymous body to
// discover wire field names.
//
// `KnownIntentionallySkipped` returns wire-side JSON names that should
// NOT raise a coverage error even when no schema attribute covers them.
// Keep it small and per-resource. Typical entries are computed-only
// fields from a different conceptual domain (e.g. `authorId` /
// `authorName` on an agent — surfaced by the API but not part of how
// users manage agents in Terraform), or fields that are duplicated by
// existing typed accessors.
type resourceWithAPIShape interface {
	APIShape() any
	KnownIntentionallySkipped() []string
}

// TestApiCoverage asserts every wire field on the GET response has a
// corresponding schema attribute (or is on the per-resource skip list).
// Catches the silent-drift class of bug where the backend adds a field
// and the provider keeps shipping without surfacing it — users get no
// drift signal from `terraform plan` and can't read the field at all.
//
// The test mirrors `TestSpecDrift`'s opt-in pattern: only resources
// implementing `resourceWithAPIShape` are checked. New resources are
// auto-covered the moment they implement the method.
//
// Schema-level coverage is the right granularity, not AttrSpec-level —
// AttrSpec is the merge-patch wire metadata (fields the provider _sends_),
// and many API fields are read-only (`createdAt`, deprecated columns)
// that should be exposed as Computed-only schema attrs without an
// AttrSpec entry. `TestSpecDrift` already enforces schema↔AttrSpec
// alignment in the other direction, so an Optional/Required schema attr
// without an AttrSpec entry will be caught there.
func TestApiCoverage(t *testing.T) {
	t.Parallel()

	prov := New("test")()
	ctx := t.Context()

	for _, ctor := range prov.Resources(ctx) {
		r := ctor()

		shaped, ok := r.(resourceWithAPIShape)
		if !ok {
			continue
		}

		var metaResp resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "archestra"}, &metaResp)

		t.Run(metaResp.TypeName, func(t *testing.T) {
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			schemaSet := make(map[string]struct{})
			for _, n := range topLevelSchemaAttrNames(schemaResp.Schema) {
				schemaSet[n] = struct{}{}
			}

			// AttrSpec.JSONName maps wire keys to TF attrs whose names
			// don't snake-roundtrip (e.g. `customFont` ↔ `font`,
			// `theme` ↔ `color_theme`). Honor those declared mappings
			// as covered.
			attrSpecJSONSet := make(map[string]struct{})
			if specs, ok := r.(resourceWithAttrSpec); ok {
				for _, s := range specs.AttrSpecs() {
					if s.JSONName != "" {
						attrSpecJSONSet[s.JSONName] = struct{}{}
					}
				}
			}

			skipSet := make(map[string]struct{})
			for _, n := range shaped.KnownIntentionallySkipped() {
				skipSet[n] = struct{}{}
			}

			apiFields := walkAPIShape(shaped.APIShape())
			sort.Strings(apiFields)

			for _, jsonName := range apiFields {
				if _, ok := skipSet[jsonName]; ok {
					continue
				}
				if _, ok := attrSpecJSONSet[jsonName]; ok {
					continue
				}
				if _, ok := schemaSet[camelToSnake(jsonName)]; ok {
					continue
				}
				t.Errorf("API wire field %q has no matching schema attribute (snake-case lookup %q) "+
					"and no AttrSpec entry maps it. Either expose it as a schema attribute "+
					"(Computed-only is fine for read-only fields), or add %q to "+
					"KnownIntentionallySkipped() with a justification comment.",
					jsonName, camelToSnake(jsonName), jsonName)
			}
		})
	}
}

// walkAPIShape extracts the top-level JSON field names from the per-record
// body of a generated Get response. Accepts the response struct directly
// (e.g. `client.GetFooResponse{}`); finds the `JSON200` field, drills
// through pointers and slice elements, and unwraps a paginated `{Data
// []record, Pagination ...}` envelope when present.
func walkAPIShape(shape any) []string {
	t := reflect.TypeOf(shape)
	if t == nil {
		return nil
	}
	// Locate the response body type: `Get<Op>Response.JSON200`.
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	bodyField, ok := t.FieldByName("JSON200")
	if !ok {
		return nil
	}
	t = bodyField.Type

	// Drill through pointer/slice wrappers and the paginated `{Data, Pagination}`
	// envelope until we reach the per-record struct.
	for {
		if t == nil {
			return nil
		}
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice:
			t = t.Elem()
			continue
		case reflect.Struct:
			dataField, hasData := t.FieldByName("Data")
			_, hasPagination := t.FieldByName("Pagination")
			if hasData && hasPagination && dataField.Type.Kind() == reflect.Slice {
				t = dataField.Type.Elem()
				continue
			}
		default:
			return nil
		}
		break
	}

	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if comma := strings.Index(tag, ","); comma >= 0 {
			tag = tag[:comma]
		}
		if tag == "" {
			continue
		}
		out = append(out, tag)
	}
	return out
}

func TestCamelToSnake(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"id", "id"},
		{"name", "name"},
		{"createdAt", "created_at"},
		{"defaultLlmModel", "default_llm_model"},
		{"mcpOauthAccessTokenLifetimeSeconds", "mcp_oauth_access_token_lifetime_seconds"},
		{"defaultLlmApiKeyId", "default_llm_api_key_id"},
		// Trailing acronym collapses to one word — matches the schema names
		// the codebase actually uses (e.g. `entity_id` for `entityID`).
		{"entityURL", "entity_url"},
		{"entityID", "entity_id"},
		{"", ""},
		{"X", "x"},
	}
	for _, tc := range cases {
		if got := camelToSnake(tc.in); got != tc.want {
			t.Errorf("camelToSnake(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWalkAPIShape(t *testing.T) {
	t.Parallel()

	type singleRecordResponse struct {
		Body    []byte
		JSON200 *struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			// Empty json tag → skipped.
			Internal string ``
			// "-" tag → skipped.
			Skip string `json:"-"`
		}
	}
	got := walkAPIShape(singleRecordResponse{})
	want := map[string]bool{"id": true, "name": true}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected field %q", n)
		}
	}

	type listResponse struct {
		JSON200 *[]struct {
			Foo string `json:"foo"`
			Bar string `json:"bar,omitempty"`
		}
	}
	got = walkAPIShape(listResponse{})
	want = map[string]bool{"foo": true, "bar": true}
	if len(got) != len(want) {
		t.Fatalf("list response: expected %v, got %v", want, got)
	}

	type paginatedResponse struct {
		JSON200 *struct {
			Data []struct {
				Item string `json:"item"`
			} `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
	}
	got = walkAPIShape(paginatedResponse{})
	if len(got) != 1 || got[0] != "item" {
		t.Errorf("paginated response: expected [item], got %v", got)
	}

	// No JSON200 field → empty result, no panic.
	type bogusResponse struct{ NotJSON200 string }
	if got := walkAPIShape(bogusResponse{}); got != nil {
		t.Errorf("bogus response: expected nil, got %v", got)
	}
}

// camelToSnake converts a camelCase JSON field name to snake_case to match
// Terraform Plugin Framework's tfsdk naming convention. Inserts an
// underscore before any uppercase letter that follows a lowercase letter
// or digit, then lowercases everything. Consecutive uppercase letters
// (acronyms like the trailing `URL` in `entityURL`) collapse into a
// single trailing word — e.g. `entityURL` → `entity_url`.
//
// The terraform-provider-archestra JSON tags use Go's idiomatic
// camelCase with single-char acronyms (`Id`, `Url`, `Llm`, `Api`) most of
// the time, so the simple rule covers nearly all real cases. The
// trailing-acronym case is the one wrinkle that needs explicit handling.
func camelToSnake(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	prev := rune(0)
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
		prev = r
	}
	return b.String()
}
