package provider

import (
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// resourceWithAttrSpec is implemented by any resource that exposes its merge-
// patch wire metadata. Resources that haven't been migrated yet won't satisfy
// this interface; the drift check skips them. As resources adopt the helper,
// they implement this method and the test starts enforcing schema ↔ AttrSpec
// alignment automatically.
type resourceWithAttrSpec interface {
	AttrSpecs() []AttrSpec
}

// TestSpecDrift asserts every Schema attribute on a migrated resource has a
// matching AttrSpec entry, every AttrSpec entry references a real schema
// attribute, and every `Sensitive: true` schema attribute is marked Sensitive
// on its AttrSpec. Catches "added schema field, forgot AttrSpec" before users
// do.
//
// The test iterates the provider's registered resource constructors, so any
// new resource is auto-covered the moment it implements resourceWithAttrSpec.
func TestSpecDrift(t *testing.T) {
	t.Parallel()

	prov := New("test")()
	ctx := t.Context()

	for _, ctor := range prov.Resources(ctx) {
		r := ctor()

		var metaResp resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "archestra"}, &metaResp)

		spec, ok := r.(resourceWithAttrSpec)
		if !ok {
			continue
		}

		t.Run(metaResp.TypeName, func(t *testing.T) {
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			schemaNames := topLevelSchemaAttrNames(schemaResp.Schema)
			schemaSensitive := topLevelSensitiveAttrNames(schemaResp.Schema)
			specNames := attrSpecTFNames(spec.AttrSpecs())
			specSensitive := attrSpecSensitiveTFNames(spec.AttrSpecs())

			// Schema attribute missing from AttrSpec.
			for _, name := range diff(schemaNames, specNames) {
				if isComputedOnly(schemaResp.Schema, name) {
					continue
				}
				t.Errorf("schema attribute %q has no matching AttrSpec entry", name)
			}

			// AttrSpec entry has no matching schema attribute (typo, stale entry).
			for _, name := range diff(specNames, schemaNames) {
				t.Errorf("AttrSpec entry %q has no matching schema attribute", name)
			}

			// Sensitive disagreement.
			for _, name := range schemaSensitive {
				if !contains(specSensitive, name) {
					t.Errorf("schema attribute %q has Sensitive: true but AttrSpec entry is not marked Sensitive", name)
				}
			}
			for _, name := range specSensitive {
				if !contains(schemaSensitive, name) {
					t.Errorf("AttrSpec entry %q is marked Sensitive but schema attribute is not", name)
				}
			}
		})
	}
}

// topLevelSchemaAttrNames returns the names of all top-level Attributes and
// Blocks declared by the schema. Computed-only attributes (id, created_at,
// etc.) are still returned; the caller filters them with isComputedOnly.
func topLevelSchemaAttrNames(s rschema.Schema) []string {
	names := make([]string, 0, len(s.Attributes)+len(s.Blocks))
	for name := range s.Attributes {
		names = append(names, name)
	}
	for name := range s.Blocks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func topLevelSensitiveAttrNames(s rschema.Schema) []string {
	out := []string{}
	for name, attr := range s.Attributes {
		if attr.IsSensitive() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func isComputedOnly(s rschema.Schema, name string) bool {
	attr, ok := s.Attributes[name]
	if !ok {
		return false
	}
	return attr.IsComputed() && !attr.IsRequired() && !attr.IsOptional()
}

func attrSpecTFNames(specs []AttrSpec) []string {
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.TFName)
	}
	sort.Strings(out)
	return out
}

func attrSpecSensitiveTFNames(specs []AttrSpec) []string {
	out := []string{}
	for _, s := range specs {
		if s.Sensitive {
			out = append(out, s.TFName)
		}
	}
	sort.Strings(out)
	return out
}

// diff returns elements in `a` that are not in `b`.
func diff(a, b []string) []string {
	bset := make(map[string]struct{}, len(b))
	for _, x := range b {
		bset[x] = struct{}{}
	}
	out := []string{}
	for _, x := range a {
		if _, ok := bset[x]; !ok {
			out = append(out, x)
		}
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, x := range haystack {
		if x == needle {
			return true
		}
	}
	return false
}
