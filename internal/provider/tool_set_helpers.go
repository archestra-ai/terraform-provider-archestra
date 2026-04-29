package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// parseUUIDSet converts a Terraform `types.Set` of strings to a slice of
// `openapi_types.UUID`, recording per-element parse errors against the
// caller's diagnostics. Used by the bulk tool resources.
func parseUUIDSet(ctx context.Context, set types.Set, diags *diag.Diagnostics, field string) []openapi_types.UUID {
	var raw []string
	d := set.ElementsAs(ctx, &raw, false)
	diags.Append(d...)
	if d.HasError() {
		return nil
	}
	out := make([]openapi_types.UUID, 0, len(raw))
	for _, s := range raw {
		u, err := uuid.Parse(s)
		if err != nil {
			diags.AddError(fmt.Sprintf("Invalid UUID in %s", field), fmt.Sprintf("%q: %s", s, err))
			return nil
		}
		out = append(out, u)
	}
	return out
}

// syntheticToolSetID hashes the (sorted tool_ids, action) tuple into a
// stable resource ID. Two resources with the same set + action collide,
// which is fine — they're idempotent.
func syntheticToolSetID(tools []openapi_types.UUID, action string) string {
	if len(tools) == 0 {
		return action
	}
	strs := make([]string, len(tools))
	for i, t := range tools {
		strs[i] = t.String()
	}
	sort.Strings(strs)
	h := strs[0]
	for _, s := range strs[1:] {
		h += "," + s
	}
	return action + ":" + h
}

// diffUUIDSets returns the elements to add (in want, not in have) and to
// remove (in have, not in want). Stable order isn't guaranteed; callers
// don't depend on it.
func diffUUIDSets(want, have []openapi_types.UUID) (add, remove []openapi_types.UUID) {
	w := make(map[openapi_types.UUID]struct{}, len(want))
	for _, u := range want {
		w[u] = struct{}{}
	}
	h := make(map[openapi_types.UUID]struct{}, len(have))
	for _, u := range have {
		h[u] = struct{}{}
	}
	for u := range w {
		if _, found := h[u]; !found {
			add = append(add, u)
		}
	}
	for u := range h {
		if _, found := w[u]; !found {
			remove = append(remove, u)
		}
	}
	return
}
