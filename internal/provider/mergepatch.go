package provider

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// AttrKind classifies how a field is diffed in a JSON Merge Patch (RFC 7396).
//
// Scalar / List / Set / Map are leaf-typed fields. AtomicObject and
// RecursiveObject describe nested-object fields and differ by their backend
// storage shape:
//
//   - AtomicObject — backed by a single JSONB or TEXT-blob column. Drizzle
//     `update().set({col: x})` replaces the whole column, so any sub-field
//     change forces emission of the *whole* plan-side object. Never recurse
//     for diffing; sub-children describe the wire shape (for re-keying and
//     log masking) only.
//
//   - RecursiveObject — backed by multiple top-level columns grouped under
//     an HCL block for ergonomics. Safe to diff field-by-field; recurse with
//     each child's own AttrKind.
//
// Today every nested object in this provider is AtomicObject. RecursiveObject
// exists in the design for completeness.
type AttrKind int

const (
	// Scalar: string, bool, number, sensitive scalars.
	Scalar AttrKind = iota
	// List: ordered; replaced wholesale on change.
	List
	// Set: unordered; replaced wholesale on change.
	Set
	// Map: string-keyed; replaced wholesale on change.
	Map
	// AtomicObject: single-column JSONB / TEXT blob. Emit whole on any change.
	AtomicObject
	// RecursiveObject: multi-column group. Diff field-by-field.
	RecursiveObject
	// Synthetic: HCL-only ergonomic grouping with no direct wire field. Used
	// for schema attributes that explode into multiple top-level wire keys
	// (e.g., a `remote_config` block whose `url` and `oauth_config` materialize
	// as top-level `serverUrl` and `oauthConfig`). MergePatch skips Synthetic
	// entries entirely; the caller is responsible for materializing the wire
	// fields out-of-band.
	Synthetic
)

// AttrSpec declares one field's wire shape and diff behavior. One slice per
// resource, declared adjacent to the resource's Schema(). A drift-check test
// asserts every Schema attribute is covered by an AttrSpec entry.
type AttrSpec struct {
	// TFName is the tfsdk attribute name (snake_case as it appears in HCL).
	TFName string
	// JSONName is the wire key sent to the backend (camelCase for Archestra).
	JSONName string
	// Kind selects the diff/encoding strategy.
	Kind AttrKind
	// Children describes nested attributes:
	//   - For RecursiveObject: per-child specs to recurse into for diffing.
	//   - For AtomicObject: per-child specs used to re-key tfsdk → JSON
	//     when emitting the whole object, and to drive log masking.
	//   - For List / Set whose elements are objects: per-element-field specs.
	Children []AttrSpec
	// Encoder is an optional value transform applied after extraction.
	// E.g., normalize a UUID, JSON-encode a polymorphic string, etc.
	Encoder func(any) any
	// Sensitive mirrors the schema's `Sensitive: true`. Drives log masking.
	Sensitive bool
	// OmitOnNull, when true, causes plan-null + prior-non-null transitions
	// to be omitted from the patch instead of emitted as JSON null.
	//
	// Use for backend fields whose zod schema is `.optional()` but not
	// `.nullable().optional()` — sending null gets rejected as
	// "Invalid input: expected ..., received null". With this flag, removing
	// the attribute from HCL is interpreted as "stop managing" rather than
	// "explicitly clear", which matches the backend's actual semantics for
	// non-nullable optional fields.
	OmitOnNull bool
}

// MergePatch returns an RFC 7396 JSON Merge Patch representing the diff
// between `prior` (current state) and `plan` (proposed new state). The
// resulting map is ready to JSON-marshal as the request body for any
// merge-patch-style endpoint.
//
// For Create paths, pass `prior = tftypes.NewValue(planType, nil)` (a null
// object of the same type as plan). The diff then emits every non-null
// attribute from the plan, which is exactly Create semantics.
//
// Diff rules per AttrKind:
//   - Equal plan/prior → omitted from the patch.
//   - Plan unknown (computed-after-apply) → omitted; backend computes it.
//   - Plan null + prior non-null → emitted as JSON `null` (clears the field).
//   - Scalar / List / Set / Map / AtomicObject changed → emitted as the full
//     plan-side encoded value.
//   - RecursiveObject changed → emitted as a sub-merge-patch built from
//     children diffs.
//
// Both `plan` and `prior` must be tftypes.Value of an Object type.
func MergePatch(
	ctx context.Context,
	plan, prior tftypes.Value,
	attrs []AttrSpec,
	diags *diag.Diagnostics,
) map[string]any {
	if !plan.IsKnown() {
		// Computed-after-apply at the object level — no-op.
		return map[string]any{}
	}

	planFields := map[string]tftypes.Value{}
	if !plan.IsNull() {
		if err := plan.As(&planFields); err != nil {
			diags.AddError("MergePatch decode error", "decode plan: "+err.Error())
			return nil
		}
	}

	priorFields := map[string]tftypes.Value{}
	if !prior.IsNull() {
		if err := prior.As(&priorFields); err != nil {
			diags.AddError("MergePatch decode error", "decode prior: "+err.Error())
			return nil
		}
	}

	out := map[string]any{}
	for _, spec := range attrs {
		if spec.Kind == Synthetic {
			continue
		}

		planV, hasPlan := planFields[spec.TFName]
		priorV, hasPrior := priorFields[spec.TFName]

		if !hasPlan && !hasPrior {
			continue
		}
		if !hasPrior {
			priorV = tftypes.NewValue(planV.Type(), nil)
		}
		if !hasPlan {
			planV = tftypes.NewValue(priorV.Type(), nil)
		}

		if !planV.IsKnown() {
			continue
		}

		if planV.Equal(priorV) {
			continue
		}

		if planV.IsNull() {
			if spec.OmitOnNull {
				continue
			}
			out[spec.JSONName] = nil
			continue
		}

		switch spec.Kind {
		case Scalar, List, Set, Map, AtomicObject:
			val, err := encodeValue(planV, spec.Children)
			if err != nil {
				diags.AddError("MergePatch encode error", fmt.Sprintf("attribute %q: %s", spec.TFName, err))
				continue
			}
			if spec.Encoder != nil {
				val = spec.Encoder(val)
			}
			out[spec.JSONName] = val
		case RecursiveObject:
			sub := MergePatch(ctx, planV, priorV, spec.Children, diags)
			if len(sub) > 0 {
				out[spec.JSONName] = sub
			}
		default:
			diags.AddError("MergePatch unknown kind", fmt.Sprintf("attribute %q: unknown AttrKind %d", spec.TFName, spec.Kind))
		}
	}
	return out
}

// encodeValue converts a known, non-null tftypes.Value into a JSON-marshalable
// Go value (bool, float64, string, []any, map[string]any). When attrs is
// non-empty and the value is an Object, attrs drives the tfsdk → JSON name
// re-keying. Lists/Sets pass attrs through to describe their element shape.
func encodeValue(v tftypes.Value, attrs []AttrSpec) (any, error) {
	if !v.IsKnown() {
		return nil, fmt.Errorf("cannot encode unknown value")
	}
	if v.IsNull() {
		return nil, nil
	}

	t := v.Type()

	switch {
	case t.Is(tftypes.String):
		var s string
		if err := v.As(&s); err != nil {
			return nil, err
		}
		return s, nil
	case t.Is(tftypes.Bool):
		var b bool
		if err := v.As(&b); err != nil {
			return nil, err
		}
		return b, nil
	case t.Is(tftypes.Number):
		var f big.Float
		if err := v.As(&f); err != nil {
			return nil, err
		}
		// Try float64 first; fall back to a textual representation if exact.
		if fv, acc := f.Float64(); acc == big.Exact {
			return fv, nil
		}
		// big integers exceeding float64 precision: emit as int64 if it fits.
		if i, acc := f.Int64(); acc == big.Exact {
			return i, nil
		}
		return f.Text('f', -1), nil
	}

	if t.Is(tftypes.List{}) || t.Is(tftypes.Set{}) || t.Is(tftypes.Tuple{}) {
		var items []tftypes.Value
		if err := v.As(&items); err != nil {
			return nil, err
		}
		out := make([]any, 0, len(items))
		for _, item := range items {
			enc, err := encodeValue(item, attrs)
			if err != nil {
				return nil, err
			}
			out = append(out, enc)
		}
		return out, nil
	}

	if t.Is(tftypes.Object{}) {
		var fields map[string]tftypes.Value
		if err := v.As(&fields); err != nil {
			return nil, err
		}
		out := make(map[string]any, len(fields))
		if len(attrs) > 0 {
			// Spec-driven encoding: re-key tfsdk → JSON, skip null sub-fields,
			// recurse with each child's own children spec.
			for _, spec := range attrs {
				sub, ok := fields[spec.TFName]
				if !ok || sub.IsNull() || !sub.IsKnown() {
					continue
				}
				enc, err := encodeValue(sub, spec.Children)
				if err != nil {
					return nil, fmt.Errorf("encoding %q: %w", spec.TFName, err)
				}
				if spec.Encoder != nil {
					enc = spec.Encoder(enc)
				}
				out[spec.JSONName] = enc
			}
		} else {
			// No spec — fall back to tfsdk names (callers should provide attrs
			// when wire names differ from tfsdk names).
			for k, sub := range fields {
				if sub.IsNull() || !sub.IsKnown() {
					continue
				}
				enc, err := encodeValue(sub, nil)
				if err != nil {
					return nil, fmt.Errorf("encoding %q: %w", k, err)
				}
				out[k] = enc
			}
		}
		return out, nil
	}

	if t.Is(tftypes.Map{}) {
		var fields map[string]tftypes.Value
		if err := v.As(&fields); err != nil {
			return nil, err
		}
		out := make(map[string]any, len(fields))
		for k, sub := range fields {
			if sub.IsNull() || !sub.IsKnown() {
				continue
			}
			enc, err := encodeValue(sub, attrs)
			if err != nil {
				return nil, fmt.Errorf("encoding map key %q: %w", k, err)
			}
			out[k] = enc
		}
		return out, nil
	}

	return nil, fmt.Errorf("unsupported tftypes type %s", t)
}

// LogPatch emits a merge patch at tflog.Debug with sensitive field values
// replaced by `***`. The mask is universal — every log level, every
// environment — driven entirely by AttrSpec.Sensitive metadata. Operators
// control verbosity via TF_LOG; redaction is not environment-conditional.
func LogPatch(ctx context.Context, msg string, patch map[string]any, attrs []AttrSpec) {
	tflog.Debug(ctx, msg, map[string]any{"patch": maskSensitive(patch, attrs)})
}

// maskSensitive walks the patch and replaces values for keys whose AttrSpec
// is marked Sensitive with the literal string "***". Recurses into nested
// objects (AtomicObject / RecursiveObject) using their Children specs.
//
// Lists / Sets / Maps are not recursed into for masking: if the whole
// collection is sensitive (e.g. a list of secrets), the AttrSpec for the
// collection itself should be Sensitive — the outer mask catches it.
func maskSensitive(patch map[string]any, attrs []AttrSpec) map[string]any {
	if patch == nil {
		return nil
	}
	byJSON := make(map[string]AttrSpec, len(attrs))
	for _, spec := range attrs {
		byJSON[spec.JSONName] = spec
	}

	out := make(map[string]any, len(patch))
	for k, v := range patch {
		spec, found := byJSON[k]
		if !found {
			out[k] = v
			continue
		}
		if spec.Sensitive && v != nil {
			out[k] = "***"
			continue
		}
		if (spec.Kind == AtomicObject || spec.Kind == RecursiveObject) && v != nil {
			if subMap, ok := v.(map[string]any); ok {
				out[k] = maskSensitive(subMap, spec.Children)
				continue
			}
		}
		out[k] = v
	}
	return out
}
