package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

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
// fixed-length, opaque resource ID. The result is stable across
// invocations and bounded in length regardless of tool set size, which
// matters because some Terraform tooling truncates long IDs in displays.
//
// Two resources with the same `(tool_ids, action)` produce the same ID
// — a Terraform-state collision that surfaces as "id already exists" on
// the second apply. That's the right outcome: writing the same default
// policy twice is a configuration mistake, and one ID per logical
// configuration makes it impossible to silently shadow.
func syntheticToolSetID(tools []openapi_types.UUID, action string) string {
	strs := make([]string, len(tools))
	for i, t := range tools {
		strs[i] = t.String()
	}
	sort.Strings(strs)
	h := sha256.New()
	h.Write([]byte(action))
	h.Write([]byte{0})
	h.Write([]byte(strings.Join(strs, ",")))
	return action + ":" + hex.EncodeToString(h.Sum(nil))
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
