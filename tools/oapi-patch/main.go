// oapi-patch normalizes the Archestra OpenAPI spec before it reaches
// oapi-codegen.
//
// Two upstream patterns make oapi-codegen emit
// `struct { union json.RawMessage }` with no helper methods, which then
// either crashes at unmarshal (numeric-arm wires) or silently drops data
// (object-arm wires, since the `union` field is unexported and json
// ignores it):
//
//  1. Inline same-Go-type union arms (e.g. embeddingDimensions:
//     `anyOf: [{type:number,enum:[3072]}, {type:number,enum:[1536]},
//     {type:number,enum:[768]}]`) — all arms collapse to float32 in Go,
//     so codegen bails on generating discriminator helpers.
//  2. Inline polymorphic unions without a discriminator (e.g. catalog
//     env.default: `anyOf: [{type:string},{type:number},{type:boolean}]`)
//     — codegen handles the same shape correctly when it appears as a
//     named `$ref` (cf. UserConfigFieldDefault), but fails on inline copies.
//
// This tool walks the spec and rewrites those two patterns:
//
//   - Same-primitive arms collapse into a single
//     `{type, enum:[union of values]}` schema (preserving nullable,
//     description, format from the parent). Numeric values shaped as
//     integers promote `type:"number"` to `type:"integer"`.
//   - Other inline unions become free-form `{nullable: …}` schemas, so
//     oapi-codegen emits `*interface{}` / `*map[string]any`. Provider code
//     that needs strict typing parses `apiResp.Body` directly — the
//     existing pattern for catalog imagePullSecrets, agent
//     builtInAgentConfig, and optimization-rule conditions.
//
// Unions whose arms are all `$ref`s to named component schemas are left
// alone; codegen's named-union path generates working helpers.
//
// TODO: This is a workaround. The real fix lives in the platform repo:
// replace inline `z.union([z.literal(N),...])` (e.g.
// EmbeddingDimensionsSchema) and inline polymorphic
// `z.union([string,number,boolean])` (e.g. catalog
// localConfig.environment[].default) with named, `.openapi()`-annotated
// schemas. Once those land and a release ships, drop this binary and
// revert codegen-api-client in the Makefile to call oapi-codegen directly.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

func main() {
	in := flag.String("in", "-", "input OpenAPI JSON (- = stdin)")
	out := flag.String("out", "-", "output OpenAPI JSON (- = stdout)")
	flag.Parse()

	src, err := readAll(*in)
	if err != nil {
		fatal("read %s: %v", *in, err)
	}

	var doc any
	if err := json.Unmarshal(src, &doc); err != nil {
		fatal("parse: %v", err)
	}

	stats := &patchStats{}
	doc = patch(doc, stats)

	w := getOut(*out)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		fatal("encode: %v", err)
	}
	if c, ok := w.(io.Closer); ok && w != os.Stdout {
		_ = c.Close()
	}

	fmt.Fprintf(os.Stderr,
		"oapi-patch: collapsed=%d freeFormed=%d unwrapped=%d preserved=%d\n",
		stats.collapsed, stats.freeFormed, stats.unwrapped, stats.preserved)
}

type patchStats struct {
	collapsed  int // same-primitive-Go-type → enum
	freeFormed int // inline mixed/object → free-form
	unwrapped  int // single-arm union → arm
	preserved  int // all-$ref union or discriminator-tagged, kept as-is
}

func patch(node any, stats *patchStats) any {
	switch n := node.(type) {
	case map[string]any:
		// Recurse first so children normalize before parent decides.
		for k, v := range n {
			n[k] = patch(v, stats)
		}
		// A schema-shaped node carries anyOf/oneOf at the top level.
		// In practice only one is set per schema.
		for _, kw := range []string{"anyOf", "oneOf"} {
			if arms, ok := n[kw].([]any); ok {
				replaceUnion(n, kw, arms, stats)
				break
			}
		}
		return n
	case []any:
		for i, v := range n {
			n[i] = patch(v, stats)
		}
		return n
	default:
		return node
	}
}

func replaceUnion(parent map[string]any, kw string, arms []any, stats *patchStats) {
	if _, hasDisc := parent["discriminator"]; hasDisc {
		stats.preserved++
		return
	}
	if len(arms) == 0 {
		delete(parent, kw)
		return
	}
	if len(arms) == 1 {
		if a, ok := arms[0].(map[string]any); ok {
			delete(parent, kw)
			for k, v := range a {
				if _, exists := parent[k]; !exists {
					parent[k] = v
				}
			}
			stats.unwrapped++
		}
		return
	}

	allRefs := true
	for _, a := range arms {
		am, ok := a.(map[string]any)
		if !ok || am["$ref"] == nil {
			allRefs = false
			break
		}
	}
	if allRefs {
		stats.preserved++
		return
	}

	// Classify arms. Three buckets:
	//  - Object-only unions (z.discriminatedUnion, customServerConfig, …):
	//    preserve. oapi-codegen emits a broken `union json.RawMessage`
	//    struct; the provider raw-parses around it where consumed.
	//  - Inline same-primitive unions (embeddingDimensions): collapse to
	//    `{type, enum: […]}`.
	//  - Anything else inline (mixed-primitive, primitive+array,
	//    primitive+object): drop to free-form. oapi-codegen then emits
	//    `*interface{}`, which JSON-roundtrips correctly.
	//
	// `array` is treated as non-primitive for the collapse decision but
	// triggers free-form rather than preserve. Object+array unions are
	// rare in practice; primitive+array (e.g. `[string, array<string>]`)
	// would otherwise resurrect the broken-union pattern silently.
	allObject := true
	allSamePrimitive := true
	primType := ""
	for _, a := range arms {
		am, _ := a.(map[string]any)
		t, _ := am["type"].(string)
		if t != "object" {
			allObject = false
		}
		if !isPrimitive(t) {
			allSamePrimitive = false
			continue
		}
		if primType == "" {
			primType = t
		} else if primType != t {
			allSamePrimitive = false
		}
	}
	if allObject {
		stats.preserved++
		return
	}

	if allSamePrimitive {
		// Collapse arms into a single {type, enum} schema. Preserves the
		// constraint set when every arm carries const/enum; falls back to
		// open primitive when any arm is unconstrained.
		merged := map[string]any{"type": primType}
		propagateMeta(parent, merged)

		var enumVals []any
		seen := map[string]bool{}
		hasOpenArm := false
		for _, a := range arms {
			am, _ := a.(map[string]any)
			if cs, ok := am["const"]; ok {
				appendUnique(&enumVals, seen, cs)
				continue
			}
			if es, ok := am["enum"].([]any); ok {
				for _, ev := range es {
					appendUnique(&enumVals, seen, ev)
				}
				continue
			}
			hasOpenArm = true
		}
		if !hasOpenArm && len(enumVals) > 0 {
			sort.SliceStable(enumVals, func(i, j int) bool {
				return fmt.Sprintf("%v", enumVals[i]) < fmt.Sprintf("%v", enumVals[j])
			})
			merged["enum"] = enumVals
		}
		// Promote integer-shaped numbers — the wire is whole numbers, the
		// idiomatic Go type is int (not float32).
		if primType == "number" && allIntegers(enumVals) {
			merged["type"] = "integer"
		}

		delete(parent, kw)
		for k, v := range merged {
			parent[k] = v
		}
		stats.collapsed++
		return
	}

	// Mixed-primitive inline union (e.g. catalog env.default
	// `string|number|boolean`). Drop to free-form so oapi-codegen emits
	// *interface{}; the provider's existing json.Marshal-then-decode
	// flow already handles polymorphic scalars correctly once the field
	// actually carries the wire value.
	delete(parent, kw)
	for _, k := range []string{"properties", "required", "items", "additionalProperties", "type"} {
		delete(parent, k)
	}
	stats.freeFormed++
}

func appendUnique(out *[]any, seen map[string]bool, v any) {
	key := fmt.Sprintf("%T:%v", v, v)
	if seen[key] {
		return
	}
	seen[key] = true
	*out = append(*out, v)
}

func isPrimitive(t string) bool {
	switch t {
	case "string", "number", "integer", "boolean":
		return true
	}
	return false
}

func propagateMeta(src, dst map[string]any) {
	for _, k := range []string{"nullable", "description", "format", "default", "example", "title"} {
		if v, ok := src[k]; ok {
			dst[k] = v
		}
	}
}

func allIntegers(vs []any) bool {
	if len(vs) == 0 {
		return false
	}
	for _, v := range vs {
		f, ok := v.(float64)
		if !ok {
			return false
		}
		if f != float64(int64(f)) {
			return false
		}
	}
	return true
}

func readAll(p string) ([]byte, error) {
	if p == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(p)
}

func getOut(p string) io.Writer {
	if p == "-" {
		return os.Stdout
	}
	f, err := os.Create(p)
	if err != nil {
		fatal("create %s: %v", p, err)
	}
	return f
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "oapi-patch: "+format+"\n", args...)
	os.Exit(1)
}
