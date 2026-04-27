package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
		{TFName: "environment", JSONName: "environment", Kind: Map},
		{TFName: "mounted_env_keys", JSONName: "mountedEnvKeys", Kind: Set},
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
		{TFName: "default", JSONName: "default", Kind: Scalar, Encoder: parseUserConfigDefault},
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

// parseUserConfigDefault transforms a user_config.default JSON-encoded string
// into its native polymorphic value. Falls back to the raw string for unquoted
// HCL input (`default = "my-value"`).
func parseUserConfigDefault(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err == nil {
		return parsed
	}
	return s
}

// finalizeCatalogItemPatch reshapes the merge-patch produced by MergePatch into
// the wire shape required by the backend. Two transforms happen here, both
// driven by HCL ergonomics that don't match the wire 1:1:
//
//  1. localConfig environment + mountedEnvKeys collapse into a single
//     `environment` array of `{key, value, type, mounted}` entries.
//  2. remote_config (HCL-only) decomposes into top-level `serverUrl` plus
//     `oauthConfig`. The diff is computed manually here because remote_config
//     is Synthetic in catalogItemAttrSpec.
//
// `serverName` is needed because oauthConfig embeds the catalog item's name
// inside the OAuth payload (server-side requirement).
func finalizeCatalogItemPatch(
	ctx context.Context,
	patch map[string]any,
	plan, prior tftypes.Value,
	serverName string,
	diags *diag.Diagnostics,
) {
	finalizeLocalConfigInPatch(patch)
	finalizeRemoteConfigInPatch(ctx, patch, plan, prior, serverName, diags)
}

// finalizeLocalConfigInPatch reshapes localConfig.environment + mountedEnvKeys
// into the wire array form, and normalizes imagePullSecrets entries.
func finalizeLocalConfigInPatch(patch map[string]any) {
	lcRaw, ok := patch["localConfig"]
	if !ok || lcRaw == nil {
		return
	}
	lc, ok := lcRaw.(map[string]any)
	if !ok {
		return
	}

	envMap, _ := lc["environment"].(map[string]any)
	mountedArr, _ := lc["mountedEnvKeys"].([]any)
	if envMap != nil || mountedArr != nil {
		mounted := make(map[string]bool, len(mountedArr))
		for _, k := range mountedArr {
			if s, ok := k.(string); ok {
				mounted[s] = true
			}
		}
		envArray := make([]map[string]any, 0, len(envMap))
		for k, v := range envMap {
			entry := map[string]any{
				"key":                  k,
				"value":                v,
				"type":                 "plain_text",
				"promptOnInstallation": false,
			}
			if mounted[k] {
				entry["mounted"] = true
			}
			envArray = append(envArray, entry)
		}
		if len(envArray) > 0 {
			lc["environment"] = envArray
		} else {
			delete(lc, "environment")
		}
		delete(lc, "mountedEnvKeys")
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
