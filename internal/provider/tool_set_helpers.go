package provider

import (
	"context"
	"fmt"
	"time"

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

// reconcileDefaultPolicyTools intersects the state's managed `tool_ids`
// with the subset whose unconditional backend default still matches
// `stateAction`. Used by the bulk-default policy resources to drift-
// detect on Read.
//
// Behaviour mirrors aws_iam_role_policy_attachment: tools whose backend
// default has been changed out-of-band (or whose tool was deleted)
// fall out of state, surfacing as a `+ tool_id` diff on the next plan.
// Apply re-asserts the configured action by calling the bulk-upsert
// endpoint with the full HCL set.
//
// `defaults` is the per-tool action map for entries where conditions=[];
// callers build it once from the list endpoint and pass it in. If two
// rows for the same tool exist (shouldn't happen — bulk-upsert should
// keep one per (tool, kind)), the later entry wins which is fine for
// drift detection purposes.
func reconcileDefaultPolicyTools(stateTools []openapi_types.UUID, stateAction string, defaults map[openapi_types.UUID]string) []openapi_types.UUID {
	kept := make([]openapi_types.UUID, 0, len(stateTools))
	for _, t := range stateTools {
		if defaults[t] == stateAction {
			kept = append(kept, t)
		}
	}
	return kept
}

// reconcileDefaultPolicyToolsWithRetry retries the GET when reconciliation
// would prune — guards against post-apply backend write-visibility lag where
// just-written rows aren't in the next GET. Fast-path on first GET when no
// prune is needed; bounded retry budget so real drift still prunes.
func reconcileDefaultPolicyToolsWithRetry(
	ctx context.Context,
	stateTools []openapi_types.UUID,
	stateAction string,
	listDefaults func(context.Context) (map[openapi_types.UUID]string, error),
) ([]openapi_types.UUID, error) {
	const (
		maxAttempts = 4
		initialWait = 250 * time.Millisecond
		maxWait     = 1 * time.Second
	)

	var lastKept []openapi_types.UUID
	wait := initialWait
	for attempt := 0; attempt < maxAttempts; attempt++ {
		defaults, err := listDefaults(ctx)
		if err != nil {
			return nil, err
		}
		kept := reconcileDefaultPolicyTools(stateTools, stateAction, defaults)
		lastKept = kept
		if len(kept) == len(stateTools) {
			return kept, nil
		}
		if attempt == maxAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		wait *= 2
		if wait > maxWait {
			wait = maxWait
		}
	}
	return lastKept, nil
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
