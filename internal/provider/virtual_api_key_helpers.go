package provider

import (
	"context"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// virtualApiKeyAttrSpec covers the body wire fields shared by
// CreateVirtualApiKey and UpdateVirtualApiKey. `llm_provider_api_key_id`
// is a URL path param (RequiresReplace) and `expires_at` is wire-typed
// as `interface{}` because the backend zod is `.nullable().optional()`
// — merge-patch emits the ISO 8601 string verbatim or `null` to clear.
var virtualApiKeyAttrSpec = []AttrSpec{
	{TFName: "llm_provider_api_key_id", JSONName: "chatApiKeyId", Kind: Synthetic},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "expires_at", JSONName: "expiresAt", Kind: Scalar},
	{TFName: "scope", JSONName: "scope", Kind: Scalar},
	{TFName: "teams", JSONName: "teams", Kind: List},
	// value is Computed+Sensitive (returned once at Create, never in
	// Update/Read). Synthetic so MergePatch ignores it; the entry
	// exists so TestSpecDrift's sensitive-marker check passes.
	{TFName: "value", Kind: Synthetic, Sensitive: true},
}

func (r *VirtualApiKeyResource) AttrSpecs() []AttrSpec {
	return virtualApiKeyAttrSpec
}

// APIShape uses the list endpoint with parent-info because the singular
// `GET /api/llm-provider-api-keys/:id/virtual-keys/:vid` route doesn't
// exist; Read paginates `/api/llm-virtual-keys` and finds by id.
func (r *VirtualApiKeyResource) APIShape() any {
	return client.GetAllVirtualApiKeysResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - tokenStart: backend-derived prefix used in the UI list view. The
//     full `value` is the user-facing secret; surfacing both would create
//     a duplicate-display state field.
//   - parentKeyBaseUrl/parentKeyName/parentKeyProvider: denormalised
//     parent metadata included for UI convenience; users already have
//     this via the parent `archestra_llm_provider_api_key` resource.
func (r *VirtualApiKeyResource) KnownIntentionallySkipped() []string {
	return []string{
		"tokenStart",
		"parentKeyBaseUrl", "parentKeyName", "parentKeyProvider",
	}
}

// expiresAtFromTime normalises a backend `*time.Time` into the state's
// RFC 3339 string representation. A nil timestamp clears the field.
func expiresAtFromTime(target *types.String, v *time.Time) {
	if v != nil {
		*target = types.StringValue(v.Format(time.RFC3339))
		return
	}
	if !target.IsNull() {
		*target = types.StringNull()
	}
}

// lastUsedAtFromTime mirrors expiresAtFromTime; separate name keeps
// call-site intent obvious.
func lastUsedAtFromTime(target *types.String, v *time.Time) {
	expiresAtFromTime(target, v)
}

// teamIDsToList projects an API team slice into a `types.List` of IDs.
// Preserves the null-vs-[] distinction across refresh: an empty API
// result against a null prior stays null, against an explicit []
// stays as an empty list.
func teamIDsToList(ctx context.Context, prior types.List, ids []string) (types.List, diag.Diagnostics) {
	if len(ids) == 0 && prior.IsNull() {
		return types.ListNull(types.StringType), nil
	}
	if ids == nil {
		ids = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, ids)
}

// teamIDsFromCreateUpdate adapts the inline-struct team slice returned
// by Create/Update into a `types.List` of IDs.
func teamIDsFromCreateUpdate(ctx context.Context, prior types.List, teams []struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}, diags *diag.Diagnostics) types.List {
	ids := make([]string, len(teams))
	for i, t := range teams {
		ids[i] = t.Id
	}
	list, d := teamIDsToList(ctx, prior, ids)
	diags.Append(d...)
	return list
}

// rfc3339TimeValidator rejects values that aren't parseable as RFC 3339
// at plan time, so a typo doesn't burn an apply round-trip.
type rfc3339TimeValidator struct{}

func (rfc3339TimeValidator) Description(_ context.Context) string {
	return "value must be an RFC 3339 timestamp (e.g. 2027-01-01T00:00:00Z)"
}

func (v rfc3339TimeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v rfc3339TimeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := time.Parse(time.RFC3339, req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid timestamp",
			"value must be an RFC 3339 timestamp (e.g. 2027-01-01T00:00:00Z): "+err.Error(),
		)
	}
}
