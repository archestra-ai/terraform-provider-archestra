package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// uuidRegexp matches RFC 4122-style UUIDs in either case. The backend
// emits lowercase, but values copy-pasted from a browser UI may be
// uppercase, and Go's `uuid.Parse` is case-insensitive too — so the
// validator should be lenient about the case it accepts.
var uuidRegexp = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// jsonObject is a validator.String that fails plan when the configured value,
// parsed as JSON, is not a JSON object. Null/unknown values are ignored so it
// composes with Optional attributes.
//
// jsontypes.Normalized already ensures the value is valid JSON; this layer
// enforces "must be an object".
type jsonObject struct{}

// jsonObjectValidator returns the jsonObject validator. Kept package-internal
// because the only consumer is resource_identity_provider.go.
func jsonObjectValidator() validator.String {
	return jsonObject{}
}

func (v jsonObject) Description(_ context.Context) string {
	return "value must be a JSON object (e.g. `jsonencode({ key = \"val\" })`)"
}

func (v jsonObject) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v jsonObject) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	raw := req.ConfigValue.ValueString()
	if raw == "" {
		return
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON",
			fmt.Sprintf("Unable to parse value as JSON: %s", err),
		)
		return
	}
	if _, ok := decoded.(map[string]interface{}); !ok {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"JSON value must be an object",
			fmt.Sprintf("Expected a JSON object; got %T. Use `jsonencode({ key = value })` to produce an object.", decoded),
		)
	}
}
