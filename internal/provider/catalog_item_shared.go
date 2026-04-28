package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// catalogItemAttrSpec describes the wire shape of archestra_mcp_registry_catalog_item.
//
// Wire-naming quirks worth noting:
//   - Top-level keys are camelCase (`docsUrl`, `requiresAuth`).
//   - `localConfig` is a JSONB column (AtomicObject); its sub-keys are camelCase.
//   - `enterpriseManagedConfig` is a JSONB column (AtomicObject), camelCase sub-keys.
//   - `userConfig` is a JSONB column shaped as `Map<string, Object>` whose
//     `default` sub-field is a polymorphic union (string | number | bool | []string).
//     The HCL side stores `default` as a JSON-encoded string for round-trip;
//     the per-field Encoder JSON-decodes it on the way out.
//   - `remote_config` is HCL-only ergonomics; on the wire it explodes into
//     top-level `serverUrl` plus `oauthConfig` (which itself uses snake_case
//     keys, NOT camelCase). The Synthetic Kind keeps it out of the merge-patch
//     diff path; finalizeCatalogItemPatch materializes the wire fields by
//     diffing plan vs prior remote_config out-of-band.
//   - `localConfig.environment` (HCL `Map<string,string>`) plus
//     `localConfig.mountedEnvKeys` (HCL `Set<string>`) collapse into a single
//     wire array `localConfig.environment` of `{key, value, type, mounted, ...}`
//     entries. Because `localConfig` is AtomicObject (JSONB column), any change
//     to either field forces emission of the whole object — finalizeCatalogItemPatch
//     reshapes the post-MergePatch `environment` and drops `mountedEnvKeys`.
//   - `localConfig.imagePullSecrets` has mode-dependent fields driven by `source`;
//     finalizeCatalogItemPatch normalizes each entry to the wire shape.
var catalogItemAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "docs_url", JSONName: "docsUrl", Kind: Scalar},
	{TFName: "installation_command", JSONName: "installationCommand", Kind: Scalar},
	{TFName: "auth_description", JSONName: "authDescription", Kind: Scalar},
	{TFName: "version", JSONName: "version", Kind: Scalar},
	{TFName: "repository", JSONName: "repository", Kind: Scalar},
	{TFName: "instructions", JSONName: "instructions", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "requires_auth", JSONName: "requiresAuth", Kind: Scalar},
	{TFName: "deployment_spec_yaml", JSONName: "deploymentSpecYaml", Kind: Scalar},
	{TFName: "scope", JSONName: "scope", Kind: Scalar},
	{TFName: "teams", JSONName: "teams", Kind: List},
	{TFName: "labels", JSONName: "labels", Kind: List, Children: []AttrSpec{
		{TFName: "key", JSONName: "key", Kind: Scalar},
		{TFName: "value", JSONName: "value", Kind: Scalar},
	}},
	{TFName: "auth_fields", JSONName: "authFields", Kind: List, Children: []AttrSpec{
		{TFName: "name", JSONName: "name", Kind: Scalar},
		{TFName: "label", JSONName: "label", Kind: Scalar},
		{TFName: "type", JSONName: "type", Kind: Scalar},
		{TFName: "required", JSONName: "required", Kind: Scalar},
		{TFName: "description", JSONName: "description", Kind: Scalar},
	}},

	{TFName: "client_secret_id", JSONName: "clientSecretId", Kind: Scalar},
	{TFName: "local_config_secret_id", JSONName: "localConfigSecretId", Kind: Scalar},
	{TFName: "local_config_vault_key", JSONName: "localConfigVaultKey", Kind: Scalar},
	{TFName: "local_config_vault_path", JSONName: "localConfigVaultPath", Kind: Scalar},
	{TFName: "oauth_client_secret_vault_key", JSONName: "oauthClientSecretVaultKey", Kind: Scalar},
	{TFName: "oauth_client_secret_vault_path", JSONName: "oauthClientSecretVaultPath", Kind: Scalar},

	{TFName: "enterprise_managed_config", JSONName: "enterpriseManagedConfig", Kind: AtomicObject, Children: []AttrSpec{
		{TFName: "identity_provider_id", JSONName: "identityProviderId", Kind: Scalar},
		{TFName: "resource_type", JSONName: "resourceType", Kind: Scalar},
		{TFName: "resource_identifier", JSONName: "resourceIdentifier", Kind: Scalar},
		{TFName: "requested_issuer", JSONName: "requestedIssuer", Kind: Scalar},
		{TFName: "requested_credential_type", JSONName: "requestedCredentialType", Kind: Scalar},
		{TFName: "scopes", JSONName: "scopes", Kind: List},
		{TFName: "audience", JSONName: "audience", Kind: Scalar},
		{TFName: "client_id_override", JSONName: "clientIdOverride", Kind: Scalar},
		{TFName: "token_injection_mode", JSONName: "tokenInjectionMode", Kind: Scalar},
		{TFName: "header_name", JSONName: "headerName", Kind: Scalar},
		{TFName: "env_var_name", JSONName: "envVarName", Kind: Scalar},
		{TFName: "body_field_name", JSONName: "bodyFieldName", Kind: Scalar},
		{TFName: "response_field_path", JSONName: "responseFieldPath", Kind: Scalar},
		{TFName: "fallback_mode", JSONName: "fallbackMode", Kind: Scalar},
		{TFName: "cache_ttl_seconds", JSONName: "cacheTtlSeconds", Kind: Scalar},
		{TFName: "assertion_mode", JSONName: "assertionMode", Kind: Scalar},
	}},

	{TFName: "local_config", JSONName: "localConfig", Kind: AtomicObject, Children: []AttrSpec{
		{TFName: "command", JSONName: "command", Kind: Scalar},
		{TFName: "arguments", JSONName: "arguments", Kind: List},
		{TFName: "environment", JSONName: "environment", Kind: Set, Children: []AttrSpec{
			{TFName: "key", JSONName: "key", Kind: Scalar},
			{TFName: "type", JSONName: "type", Kind: Scalar},
			{TFName: "value", JSONName: "value", Kind: Scalar},
			{TFName: "prompt_on_installation", JSONName: "promptOnInstallation", Kind: Scalar},
			{TFName: "required", JSONName: "required", Kind: Scalar},
			{TFName: "description", JSONName: "description", Kind: Scalar},
			{TFName: "default", JSONName: "default", Kind: Scalar},
			{TFName: "mounted", JSONName: "mounted", Kind: Scalar},
		}},
		{TFName: "env_from", JSONName: "envFrom", Kind: List, Children: []AttrSpec{
			{TFName: "type", JSONName: "type", Kind: Scalar},
			{TFName: "name", JSONName: "name", Kind: Scalar},
			{TFName: "prefix", JSONName: "prefix", Kind: Scalar},
		}},
		{TFName: "docker_image", JSONName: "dockerImage", Kind: Scalar},
		{TFName: "transport_type", JSONName: "transportType", Kind: Scalar},
		{TFName: "http_port", JSONName: "httpPort", Kind: Scalar},
		{TFName: "http_path", JSONName: "httpPath", Kind: Scalar},
		{TFName: "service_account", JSONName: "serviceAccount", Kind: Scalar},
		{TFName: "node_port", JSONName: "nodePort", Kind: Scalar},
		{TFName: "image_pull_secrets", JSONName: "imagePullSecrets", Kind: List, Children: []AttrSpec{
			{TFName: "source", JSONName: "source", Kind: Scalar},
			{TFName: "name", JSONName: "name", Kind: Scalar},
			{TFName: "server", JSONName: "server", Kind: Scalar},
			{TFName: "username", JSONName: "username", Kind: Scalar},
			{TFName: "password", JSONName: "password", Kind: Scalar, Sensitive: true},
			{TFName: "email", JSONName: "email", Kind: Scalar},
		}},
	}},

	{TFName: "user_config", JSONName: "userConfig", Kind: Map, Children: []AttrSpec{
		{TFName: "title", JSONName: "title", Kind: Scalar},
		{TFName: "description", JSONName: "description", Kind: Scalar},
		{TFName: "type", JSONName: "type", Kind: Scalar},
		{TFName: "default", JSONName: "default", Kind: Scalar},
		{TFName: "required", JSONName: "required", Kind: Scalar},
		{TFName: "sensitive", JSONName: "sensitive", Kind: Scalar},
		{TFName: "multiple", JSONName: "multiple", Kind: Scalar},
		{TFName: "min", JSONName: "min", Kind: Scalar},
		{TFName: "max", JSONName: "max", Kind: Scalar},
		{TFName: "header_name", JSONName: "headerName", Kind: Scalar},
		{TFName: "prompt_on_installation", JSONName: "promptOnInstallation", Kind: Scalar},
	}},

	{TFName: "remote_config", Kind: Synthetic},
}

// stringFromJSONScalar reports whether the JSON bytes are a quoted string and
// returns the unquoted value when so. Used by Read flatteners to keep plain
// HCL strings as `"foo"` rather than the JSON-encoded `"\"foo\""` form for
// polymorphic-default fields.
func stringFromJSONScalar(b []byte) (string, bool) {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		return s, true
	}
	return "", false
}

// decodePolymorphicDefault chooses string vs. number vs. bool vs. []string
// decoding for a HCL-stringified `default`. The wire union is the union of
// all four (`UserConfigFieldDefaultSchema` in
// platform/backend/src/types/mcp-catalog.ts); without the type gate,
// `default = "42"` for a `type = "string"` field would JSON-decode to int 42
// and ship as a number.
//
// `multiple = true` overrides the type gate to `array<string>` (the only
// array variant the wire allows; environment fields don't accept arrays at
// all, so callers from environment pass multiple=false).
//
// Empty input means "not set" — caller should drop the field.
func decodePolymorphicDefault(s, fieldType string, multiple bool) (value any, keep bool, err error) {
	if s == "" {
		return nil, false, nil
	}
	if multiple {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			return nil, false, fmt.Errorf("multiple=true expects a JSON array of strings as default, got %q (use jsonencode([\"a\",\"b\"]))", s)
		}
		return arr, true, nil
	}
	switch fieldType {
	case "string", "file", "directory", "plain_text", "secret":
		return s, true, nil
	case "number":
		var n float64
		if err := json.Unmarshal([]byte(s), &n); err != nil {
			return nil, false, fmt.Errorf("type=%q expects a numeric default, got %q (use jsonencode for non-string defaults)", fieldType, s)
		}
		return n, true, nil
	case "boolean":
		var b bool
		if err := json.Unmarshal([]byte(s), &b); err != nil {
			return nil, false, fmt.Errorf("type=%q expects a boolean default, got %q (use jsonencode(true) / jsonencode(false))", fieldType, s)
		}
		return b, true, nil
	default:
		return nil, false, fmt.Errorf("unsupported type=%q for default-value decoding", fieldType)
	}
}

// finalizeCatalogItemPatch covers two HCL-vs-wire impedance mismatches:
// `remote_config` is Synthetic and must be decomposed into top-level
// `serverUrl` + `oauthConfig` (the latter embeds the catalog item's
// `serverName` per backend requirement); and several JSONB sub-trees need
// per-entry post-encoding (polymorphic defaults, image-pull-secret discriminator).
func finalizeCatalogItemPatch(
	ctx context.Context,
	patch map[string]any,
	plan, prior tftypes.Value,
	serverName string,
	diags *diag.Diagnostics,
) {
	finalizeLocalConfigInPatch(patch, diags)
	finalizeUserConfigInPatch(patch, diags)
	finalizeRemoteConfigInPatch(ctx, patch, plan, prior, serverName, diags)
}

func finalizeUserConfigInPatch(patch map[string]any, diags *diag.Diagnostics) {
	uc, ok := patch["userConfig"].(map[string]any)
	if !ok {
		return
	}
	for key, entryRaw := range uc {
		entry, ok := entryRaw.(map[string]any)
		if !ok {
			continue
		}
		raw, has := entry["default"]
		if !has || raw == nil {
			continue
		}
		s, ok := raw.(string)
		if !ok {
			continue
		}
		fieldType, _ := entry["type"].(string)
		multiple, _ := entry["multiple"].(bool)
		v, keep, err := decodePolymorphicDefault(s, fieldType, multiple)
		if err != nil {
			diags.AddAttributeError(
				path.Root("user_config").AtMapKey(key).AtName("default"),
				"Invalid user_config default",
				err.Error(),
			)
			continue
		}
		if keep {
			entry["default"] = v
		} else {
			delete(entry, "default")
		}
	}
}

// finalizeLocalConfigInPatch fills in `environment[].promptOnInstallation`
// (wire-required, no schema default reaches this point when the entry is
// fresh-built from a partial diff), runs the polymorphic-default gate per
// entry, and discriminates each `imagePullSecrets` entry by its `source`.
func finalizeLocalConfigInPatch(patch map[string]any, diags *diag.Diagnostics) {
	lcRaw, ok := patch["localConfig"]
	if !ok || lcRaw == nil {
		return
	}
	lc, ok := lcRaw.(map[string]any)
	if !ok {
		return
	}

	if envArr, ok := lc["environment"].([]any); ok {
		for i, item := range envArr {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if _, set := entry["promptOnInstallation"]; !set {
				entry["promptOnInstallation"] = false
			}
			if raw, has := entry["default"]; has && raw != nil {
				if s, ok := raw.(string); ok {
					fieldType, _ := entry["type"].(string)
					v, keep, err := decodePolymorphicDefault(s, fieldType, false)
					if err != nil {
						diags.AddAttributeError(
							path.Root("local_config").AtName("environment").AtListIndex(i).AtName("default"),
							"Invalid environment default",
							err.Error(),
						)
						continue
					}
					if keep {
						entry["default"] = v
					} else {
						delete(entry, "default")
					}
				}
			}
		}
	}

	if ips, ok := lc["imagePullSecrets"].([]any); ok {
		normalized := make([]map[string]string, 0, len(ips))
		for _, item := range ips {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			normalized = append(normalized, normalizeImagePullSecret(entry))
		}
		lc["imagePullSecrets"] = normalized
	}
}

// normalizeImagePullSecret collapses the encoded image_pull_secrets entry into
// the source-keyed shape the backend expects: `existing` mode keeps `name`;
// `credentials` mode keeps `server`, `username`, `password`, optional `email`.
// Empty/missing fields are dropped — the backend rejects empty strings on
// `credentials` mode.
func normalizeImagePullSecret(entry map[string]any) map[string]string {
	source, _ := entry["source"].(string)
	if source == "" {
		source = "existing"
	}
	out := map[string]string{"source": source}
	if source == "credentials" {
		for _, k := range []string{"server", "username", "password", "email"} {
			if v, ok := entry[k].(string); ok && v != "" {
				out[k] = v
			}
		}
	} else {
		if v, ok := entry["name"].(string); ok && v != "" {
			out["name"] = v
		}
	}
	return out
}

// finalizeRemoteConfigInPatch decomposes any remote_config plan-vs-prior delta
// into top-level `serverUrl` and `oauthConfig`. oauthConfig is treated as
// AtomicObject (any sub-field change emits the whole thing) and uses
// snake_case wire keys per the backend's catalog item OAuth schema.
func finalizeRemoteConfigInPatch(
	ctx context.Context,
	patch map[string]any,
	plan, prior tftypes.Value,
	serverName string,
	diags *diag.Diagnostics,
) {
	planRC, _ := extractObjectField(plan, "remote_config")
	priorRC, _ := extractObjectField(prior, "remote_config")

	// Both null/unknown → nothing to materialize.
	if (planRC.IsNull() || !planRC.IsKnown()) && (priorRC.IsNull() || !priorRC.IsKnown()) {
		return
	}
	if !planRC.IsKnown() {
		return
	}

	// remote_config newly null with a non-null prior — clear both wire fields.
	if planRC.IsNull() {
		patch["serverUrl"] = nil
		patch["oauthConfig"] = nil
		return
	}

	planURL := lookupOrNull(planRC, "url")
	priorURL := lookupOrNull(priorRC, "url")
	if !planURL.Equal(priorURL) && planURL.IsKnown() {
		if planURL.IsNull() {
			patch["serverUrl"] = nil
		} else {
			var s string
			if err := planURL.As(&s); err == nil {
				patch["serverUrl"] = s
			}
		}
	}

	planOC := lookupOrNull(planRC, "oauth_config")
	priorOC := lookupOrNull(priorRC, "oauth_config")
	if !planOC.Equal(priorOC) && planOC.IsKnown() {
		if planOC.IsNull() {
			patch["oauthConfig"] = nil
		} else {
			oc, err := encodeOAuthConfig(planOC, planURL, serverName)
			if err != nil {
				diags.AddError("Failed to encode oauth_config", err.Error())
				return
			}
			patch["oauthConfig"] = oc
		}
	}
}

// lookupOrNull returns the named sub-field of an Object value, falling back to
// a null-typed value if the parent is null/unknown or the field is missing.
// The caller can then use Equal/IsNull/IsKnown safely without nil-deref risk.
func lookupOrNull(parent tftypes.Value, name string) tftypes.Value {
	if !parent.IsKnown() || parent.IsNull() {
		// Use untyped null; comparisons on this aren't meaningful but at least
		// Equal vs another null-untyped returns true (both are zero-typed).
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil)
	}
	v, ok := extractObjectField(parent, name)
	if !ok {
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil)
	}
	return v
}

// encodeOAuthConfig builds the snake_case wire shape for catalog-item OAuth.
// The backend requires `client_id`, `name`, `server_url`, plus default-empty
// arrays for `scopes`, `default_scopes`, `redirect_uris`. Optional fields are
// emitted only when present.
func encodeOAuthConfig(oc, url tftypes.Value, serverName string) (map[string]any, error) {
	if oc.IsNull() || !oc.IsKnown() {
		return nil, nil
	}
	var fields map[string]tftypes.Value
	if err := oc.As(&fields); err != nil {
		return nil, err
	}

	out := map[string]any{
		"client_id":                  "",
		"name":                       serverName,
		"redirect_uris":              []any{},
		"scopes":                     []any{},
		"default_scopes":             []any{},
		"supports_resource_metadata": false,
	}
	if !url.IsNull() && url.IsKnown() {
		var s string
		if err := url.As(&s); err == nil {
			out["server_url"] = s
		}
	}
	if out["server_url"] == nil {
		out["server_url"] = ""
	}

	mappings := []struct {
		tf, json string
	}{
		{"client_id", "client_id"},
		{"client_secret", "client_secret"},
		{"redirect_uris", "redirect_uris"},
		{"scopes", "scopes"},
		{"default_scopes", "default_scopes"},
		{"supports_resource_metadata", "supports_resource_metadata"},
		{"authorization_endpoint", "authorization_endpoint"},
		{"token_endpoint", "token_endpoint"},
		{"auth_server_url", "auth_server_url"},
		{"resource_metadata_url", "resource_metadata_url"},
		{"well_known_url", "well_known_url"},
		{"grant_type", "grant_type"},
		{"audience", "audience"},
		{"access_token_env_var", "access_token_env_var"},
		{"browser_auth", "browser_auth"},
		{"generic_oauth", "generic_oauth"},
		{"requires_proxy", "requires_proxy"},
		{"provider_name", "provider_name"},
		{"streamable_http_url", "streamable_http_url"},
		{"streamable_http_port", "streamable_http_port"},
	}
	for _, m := range mappings {
		v, ok := fields[m.tf]
		if !ok || v.IsNull() || !v.IsKnown() {
			continue
		}
		enc, err := encodeValue(v, nil)
		if err != nil {
			return nil, err
		}
		out[m.json] = enc
	}
	return out, nil
}

// extractObjectField pulls a named field from an Object tftypes.Value. Returns
// the field value plus a boolean indicating presence. A null/unknown parent
// returns (zero, false).
func extractObjectField(v tftypes.Value, name string) (tftypes.Value, bool) {
	if v.IsNull() || !v.IsKnown() {
		return tftypes.Value{}, false
	}
	if !v.Type().Is(tftypes.Object{}) {
		return tftypes.Value{}, false
	}
	var fields map[string]tftypes.Value
	if err := v.As(&fields); err != nil {
		return tftypes.Value{}, false
	}
	sub, ok := fields[name]
	return sub, ok
}
