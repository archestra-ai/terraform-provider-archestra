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

			// Sensitive disagreement (top-level + nested via dotted path).
			syntheticTops := syntheticAttrSpecTops(spec.AttrSpecs())
			nestedSchemaSensitive := nestedSensitiveSchemaPaths(schemaResp.Schema, syntheticTops)
			nestedSpecSensitive := nestedSensitiveAttrSpecPaths(spec.AttrSpecs())

			allSchemaSensitive := append(append([]string{}, schemaSensitive...), nestedSchemaSensitive...)
			allSpecSensitive := append(append([]string{}, specSensitive...), nestedSpecSensitive...)

			for _, name := range allSchemaSensitive {
				if !contains(allSpecSensitive, name) {
					t.Errorf("schema attribute %q has Sensitive: true but AttrSpec entry is not marked Sensitive", name)
				}
			}
			for _, name := range allSpecSensitive {
				if !contains(allSchemaSensitive, name) {
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

// nestedSensitiveSchemaPaths walks the schema's nested attributes
// (Single/List/Set/Map NestedAttribute) and Blocks (Single/List/Set
// NestedBlock) and returns dotted paths of every nested attribute marked
// `Sensitive: true`. Top-level attributes are NOT included — those are
// handled by topLevelSensitiveAttrNames.
//
// Synthetic AttrSpec subtrees are excluded: synthetic resources like
// `remote_config` on archestra_mcp_registry_catalog_item handle their
// own wire encoding and live outside the merge-patch system, so the
// AttrSpec deliberately omits the children.
func nestedSensitiveSchemaPaths(s rschema.Schema, syntheticTops map[string]struct{}) []string {
	var out []string
	for name, a := range s.Attributes {
		if _, skip := syntheticTops[name]; skip {
			continue
		}
		walkSchemaAttrForSensitive(name, a, &out)
	}
	for name, b := range s.Blocks {
		if _, skip := syntheticTops[name]; skip {
			continue
		}
		walkSchemaBlockForSensitive(name, b, &out)
	}
	sort.Strings(out)
	return out
}

func walkSchemaAttrForSensitive(prefix string, a rschema.Attribute, out *[]string) {
	switch v := a.(type) {
	case rschema.SingleNestedAttribute:
		for childName, child := range v.Attributes {
			path := prefix + "." + childName
			if child.IsSensitive() {
				*out = append(*out, path)
			}
			walkSchemaAttrForSensitive(path, child, out)
		}
	case rschema.ListNestedAttribute:
		for childName, child := range v.NestedObject.Attributes {
			path := prefix + "." + childName
			if child.IsSensitive() {
				*out = append(*out, path)
			}
			walkSchemaAttrForSensitive(path, child, out)
		}
	case rschema.SetNestedAttribute:
		for childName, child := range v.NestedObject.Attributes {
			path := prefix + "." + childName
			if child.IsSensitive() {
				*out = append(*out, path)
			}
			walkSchemaAttrForSensitive(path, child, out)
		}
	case rschema.MapNestedAttribute:
		for childName, child := range v.NestedObject.Attributes {
			path := prefix + "." + childName
			if child.IsSensitive() {
				*out = append(*out, path)
			}
			walkSchemaAttrForSensitive(path, child, out)
		}
	}
}

func walkSchemaBlockForSensitive(prefix string, b rschema.Block, out *[]string) {
	switch v := b.(type) {
	case rschema.SingleNestedBlock:
		for childName, child := range v.Attributes {
			path := prefix + "." + childName
			if child.IsSensitive() {
				*out = append(*out, path)
			}
			walkSchemaAttrForSensitive(path, child, out)
		}
		for childName, child := range v.Blocks {
			walkSchemaBlockForSensitive(prefix+"."+childName, child, out)
		}
	case rschema.ListNestedBlock:
		for childName, child := range v.NestedObject.Attributes {
			path := prefix + "." + childName
			if child.IsSensitive() {
				*out = append(*out, path)
			}
			walkSchemaAttrForSensitive(path, child, out)
		}
		for childName, child := range v.NestedObject.Blocks {
			walkSchemaBlockForSensitive(prefix+"."+childName, child, out)
		}
	case rschema.SetNestedBlock:
		for childName, child := range v.NestedObject.Attributes {
			path := prefix + "." + childName
			if child.IsSensitive() {
				*out = append(*out, path)
			}
			walkSchemaAttrForSensitive(path, child, out)
		}
		for childName, child := range v.NestedObject.Blocks {
			walkSchemaBlockForSensitive(prefix+"."+childName, child, out)
		}
	}
}

// syntheticAttrSpecTops returns the set of top-level AttrSpec entries
// whose Kind is Synthetic — these subtrees are intentionally not walked
// for sensitive comparison because the resource handles wire encoding
// outside the merge-patch path.
func syntheticAttrSpecTops(specs []AttrSpec) map[string]struct{} {
	out := map[string]struct{}{}
	for _, s := range specs {
		if s.Kind == Synthetic {
			out[s.TFName] = struct{}{}
		}
	}
	return out
}

// nestedSensitiveAttrSpecPaths returns dotted paths of every nested
// AttrSpec entry marked `Sensitive: true`. Top-level entries are NOT
// included — those are handled by attrSpecSensitiveTFNames.
func nestedSensitiveAttrSpecPaths(specs []AttrSpec) []string {
	var out []string
	for _, s := range specs {
		walkAttrSpecForSensitive(s.TFName, s.Children, &out)
	}
	sort.Strings(out)
	return out
}

func walkAttrSpecForSensitive(prefix string, children []AttrSpec, out *[]string) {
	for _, c := range children {
		path := prefix + "." + c.TFName
		if c.Sensitive {
			*out = append(*out, path)
		}
		walkAttrSpecForSensitive(path, c.Children, out)
	}
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
