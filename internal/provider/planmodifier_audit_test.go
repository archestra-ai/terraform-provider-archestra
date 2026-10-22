package provider

import (
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// useStateForUnknownAllowlist names (resource_type, dotted_attr_path) pairs
// where Optional+Computed+UseStateForUnknown is intentional sticky behavior.
// Every entry needs a reason — the default of RemoveOnConfigNullList is
// safer; allowlisting asserts the field has no "remove = clear" semantic.
var useStateForUnknownAllowlist = map[string]map[string]string{
	"archestra_app":               {"permissions": "Omission preserves sharing; [] clears additional grants without changing ownership."},
	"archestra_knowledge_file":    {"permissions": "Omission preserves additional sharing; explicit empty clears it while retaining ownership."},
	"archestra_environment":       {"permissions": "Omission preserves sharing; explicit empty clears it without changing ownership."},
	"archestra_plugin":            {"enabled": "Omission preserves the observed setting; explicitly configure true or false to manage it.", "permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_github_pat":        {"token": "Omission preserves the write-only stored credential; supply a new value to rotate or destroy to revoke the registration."}, //nolint:gosec // G101 false-positive: token is a schema attribute name.
	"archestra_github_app":        {"private_key": "Omission preserves the write-only stored credential; supply a new value to rotate or destroy to revoke the registration."},
	"archestra_virtual_key":       {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_oauth_client":      {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_remote_agent":      {"enabled": "Omission preserves the observed setting; explicitly set true or false to manage it.", "permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_service_account":   {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_llm_proxy_profile": {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_agent":             {"skills": "Omission preserves skill settings; explicit collections control selection.", "tools": "The full assignment set is unmanaged when omitted; an explicit empty set clears only ordinary tools, preserving subagents.", "excluded_tool_ids": "Tool exclusions are unmanaged when omitted; an explicit empty set clears them.", "subagent_ids": "Agent-owned configuration is unmanaged when omitted; an explicit empty collection clears it, verified by inline relationship acceptance.", "remote_agent_ids": "Agent-owned configuration is unmanaged when omitted; an explicit empty collection clears it, verified by inline relationship acceptance.", "excluded_subagent_ids": "Agent-owned configuration is unmanaged when omitted; an explicit empty collection clears it, verified by inline relationship acceptance.", "excluded_connector_ids": "Agent-owned configuration is unmanaged when omitted; an explicit empty collection clears it, verified by inline relationship acceptance.", "hooks": "Agent-owned configuration is unmanaged when omitted; an explicit empty collection clears it, verified by inline relationship acceptance.", "permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_mcp_gateway":       {"skills": "Omission preserves gateway skill publication; explicit collections control selection.", "subagent_ids": "Gateway-owned configuration is unmanaged when omitted; explicit empty collections clear it.", "remote_agent_ids": "Gateway-owned configuration is unmanaged when omitted; explicit empty collections clear it.", "excluded_subagent_ids": "Gateway-owned configuration is unmanaged when omitted; explicit empty collections clear it.", "excluded_connector_ids": "Gateway-owned configuration is unmanaged when omitted; explicit empty collections clear it.", "tools": "Gateway-owned configuration is unmanaged when omitted; explicit empty collections clear it.", "excluded_tool_ids": "Gateway-owned configuration is unmanaged when omitted; explicit empty collections clear it.", "permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_knowledge_base":    {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_credential":        {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},
	"archestra_skill":             {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership."},

	"archestra_project":        {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership.", "instructions": "The separate instructions file is unmanaged when omitted; only explicit empty content clears it.", "app_ids": "Project app links are unmanaged when omitted; an explicit empty set unlinks all, verified by project app acceptance."},
	"archestra_model_provider": {"permissions": "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership.", "shared": "Creation-only ownership selector; importing or omitting this field must preserve the existing owner without replacement."},
	"archestra_model": {"permissions": "Omission preserves additional sharing; explicit empty clears it without changing ownership.",
		"llm_provider": "immutable adoption selector; omitting it retains the already adopted model and its provider rather than selecting another provider with the same model name.",
	},
	"archestra_agent_tool": {
		"mcp_server_id": "credential binding chosen at Create time; remove-from-config has no clear semantic — preserve prior state.",
	},
	"archestra_mcp_registry_entry": { //nolint:gosec // G101 false-positive — nested keys are schema attribute names.
		"multitenant":            "Immutable creation selector; omission preserves the observed mode so metadata updates do not replace an existing catalog item.",
		"permissions":            "Omission leaves direct sharing unmanaged; explicit empty clears additional grants while preserving ownership.",
		"client_secret_id":       "computed when the backend auto-creates a BYOS vault reference for an inline `oauth_config.client_secret`; sticky once issued so removing the inline secret doesn't accidentally drop the stored reference.",
		"local_config_secret_id": "computed when the backend auto-creates a BYOS vault reference for inline `local_config` env values; sticky once issued so removing the inline values doesn't accidentally drop the stored reference.",
	},
	"archestra_mcp_server": { //nolint:gosec // G101 false-positive — schema attribute name.
		"secret_id":        "computed when the backend auto-creates a secret to hold inline `user_config_values` / `environment_values`; sticky once issued so removing the inline values doesn't accidentally drop the stored reference.",
		"user_id":          "Immutable personal-install owner selected on creation; omission preserves the observed owner, while another owner requires replacement.",
		"hibernation_mode": "Omission preserves the observed override; explicitly set inherit to follow the organization default.",
		"scope":            "visibility scope chosen at Create time (RequiresReplace, backend default \"personal\"); remove-from-config has no clear semantic, and planning null would replace every pre-existing install whose config never set it.",
	},
	"archestra_identity_provider": stickyOidcDerivedEndpoints(
		// Auto-derived from `discovery_endpoint` on the backend when omitted
		// from config. Sticky-from-state matches the backend semantic — the
		// values are server-authoritative, the user can override but doesn't
		// have to. Without UseStateForUnknown, every refresh would surface
		// the API-populated values as "remove from config" drift.
		"oidc_config.authorization_endpoint",
		"oidc_config.token_endpoint",
		"oidc_config.user_info_endpoint",
		"oidc_config.jwks_endpoint",
		"oidc_config.token_endpoint_authentication",
		"oidc_config.skip_discovery",
	),
	"archestra_team": {"external_group_ids": "Omission leaves the SSO mapping set unmanaged; an explicit empty set clears it.", "vault_folder": "Omission leaves the licensed Vault binding unmanaged; an explicit empty string clears it.", "roles": "Role assignments are unmanaged when omitted; an explicit empty set revokes all direct team role assignments."},
	"archestra_organization_settings": stickyOrgSettings(
		"kb_bm25_b", "kb_bm25_k1",
		"online_mcp_catalog_enabled",
		"online_skill_catalog_enabled",
		"skill_marketplace_anonymous_access",
		"allow_tool_auto_assignment",
		"ocr_chat_api_key_id",
		"ocr_model",
		"kb_contextual_retrieval_mode",
		"default_model_id",
		"default_fast_model_id",
		"default_smart_model_id",
		"default_member_role",
		"onboarding_wizard",
		"icon_logo_dark",
		"require_two_factor",
		"session_max_age_seconds",
		"oauth_access_token_lifetime_seconds",
		"connection_default_mcp_gateway_id",
		"connection_default_client_id",
		"connection_shown_client_ids",
		"connection_client_order",
		"connection_client_future_providers",
		"connection_client_provider_keys",
		"connection_skills_enabled",
		"connection_llm_proxy_enabled",
		"connection_plugins_enabled",
		"connection_runtime_handoff_enabled",
		"connection_runtime_handoff_instructions",
		"model_provider_overrides",
		"messaging_channel_overrides",
		"knowledge_connector_overrides",
		"popular_agent_overrides",
		"default_environment_name",
		"default_environment_namespace",
		"default_environment_description",
		"default_network_policy",
		"default_environment_validation_regex",
		"default_environment_trusted_image_registries",
		"mcp_idle_hibernation_enabled",
		"default_personal_agent_access_all_tools",
		"apps_hackathon_recorder_enabled",
		"new_apps_disabled_by_default",
		"new_apps_locked_by_default",
		"allow_unauthenticated_llm_proxy_requests",

		"connection_base_urls",
		// Documented contract: omitting one of these fields is sticky —
		// the merge-patch sends nothing for it and the backend value is
		// preserved (per the resource MarkdownDescription). The whole
		// org-settings surface follows that semantic; listed individually
		// to keep the allowlist explicit per the audit's convention.
		"allow_chat_file_uploads",
		"animate_chat_placeholders",
		"app_name",
		"chat_error_support_message",
		"chat_links",
		"chat_placeholders",
		"default_agent_id",
		"default_llm_api_key_id",
		"embedding_chat_api_key_id",
		"embedding_model",
		"favicon",
		"footer_text",
		"icon_logo",
		"logo",
		"logo_dark",
		"og_description",
		"onboarding_complete",
		"reranker_chat_api_key_id",
		"reranker_model",
		"slim_chat_error_ui",
	),
}

// stickyOrgSettings builds an allowlist sub-map sharing the same
// "sticky-from-state" reason for every passed-in attribute name.
func stickyOrgSettings(names ...string) map[string]string {
	const reason = "documented sticky-from-state field on archestra_organization_settings — omitting from HCL preserves the backend value via the merge-patch (per resource MarkdownDescription)."
	out := make(map[string]string, len(names))
	for _, name := range []string{"environment_defaults", "default_user_limits", "default_permissions"} {
		out[name] = reason
	}
	for name := range environmentDefaultSlots {
		out["environment_defaults."+name] = reason
	}
	for name := range defaultPermissionSlots {
		out["default_permissions."+name] = reason
	}
	for _, n := range names {
		out[n] = reason
	}
	return out
}

// stickyOidcDerivedEndpoints builds an allowlist sub-map for the OIDC
// fields the backend auto-derives from `discovery_endpoint` — sticky from
// state matches the server-authoritative semantic.
func stickyOidcDerivedEndpoints(names ...string) map[string]string {
	const reason = "auto-derived from `oidc_config.discovery_endpoint` on the backend when omitted; sticky-from-state captures the API value without surfacing it as remove-from-config drift on refresh."
	out := make(map[string]string, len(names))
	for _, n := range names {
		out[n] = reason
	}
	return out
}

// TestUseStateForUnknownAudit fails when an Optional+Computed schema attribute
// carries stock UseStateForUnknown without an allowlist entry. Forces the
// remove-from-config semantic to be deliberate. Computed-only fields are
// exempt — UseStateForUnknown is the right tool for stable id/timestamp.
func TestUseStateForUnknownAudit(t *testing.T) {
	t.Parallel()

	prov := New("test")()
	ctx := t.Context()

	for _, ctor := range prov.Resources(ctx) {
		r := ctor()

		var metaResp resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "archestra"}, &metaResp)
		typeName := metaResp.TypeName

		var schemaResp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

		t.Run(typeName, func(t *testing.T) {
			var offenders []string
			collectUseStateForUnknownOnOptionalComputed("", schemaResp.Schema.Attributes, schemaResp.Schema.Blocks, &offenders)
			sort.Strings(offenders)

			allowed := useStateForUnknownAllowlist[typeName]
			offenderSet := map[string]struct{}{}
			for _, path := range offenders {
				offenderSet[path] = struct{}{}
				if _, ok := allowed[path]; ok {
					continue
				}
				t.Errorf(
					"attribute %q is Optional+Computed and uses stock UseStateForUnknown — that silently suppresses user removal from config. "+
						"Use RemoveOnConfigNullList (or document an entry in useStateForUnknownAllowlist with a reason).",
					path,
				)
			}

			// Stale allowlist entries — typo or refactor that removed the
			// attribute should drop the entry.
			allowedPaths := make([]string, 0, len(allowed))
			for p := range allowed {
				allowedPaths = append(allowedPaths, p)
			}
			sort.Strings(allowedPaths)
			for _, p := range allowedPaths {
				if _, ok := offenderSet[p]; !ok {
					t.Errorf("allowlist entry %q on %s no longer matches an Optional+Computed+UseStateForUnknown attribute — remove it.", p, typeName)
				}
			}
		})
	}
}

// collectUseStateForUnknownOnOptionalComputed walks attributes and blocks,
// appending dotted paths of every Optional+Computed attribute carrying
// stock framework UseStateForUnknown.
func collectUseStateForUnknownOnOptionalComputed(prefix string, attrs map[string]rschema.Attribute, blocks map[string]rschema.Block, out *[]string) {
	for name, a := range attrs {
		path := joinPath(prefix, name)
		if a.IsOptional() && a.IsComputed() && hasFrameworkUseStateForUnknown(a) {
			*out = append(*out, path)
		}
		switch v := a.(type) {
		case rschema.SingleNestedAttribute:
			collectUseStateForUnknownOnOptionalComputed(path, v.Attributes, nil, out)
		case rschema.ListNestedAttribute:
			collectUseStateForUnknownOnOptionalComputed(path, v.NestedObject.Attributes, nil, out)
		case rschema.SetNestedAttribute:
			collectUseStateForUnknownOnOptionalComputed(path, v.NestedObject.Attributes, nil, out)
		case rschema.MapNestedAttribute:
			collectUseStateForUnknownOnOptionalComputed(path, v.NestedObject.Attributes, nil, out)
		}
	}
	for name, b := range blocks {
		path := joinPath(prefix, name)
		switch v := b.(type) {
		case rschema.SingleNestedBlock:
			collectUseStateForUnknownOnOptionalComputed(path, v.Attributes, v.Blocks, out)
		case rschema.ListNestedBlock:
			collectUseStateForUnknownOnOptionalComputed(path, v.NestedObject.Attributes, v.NestedObject.Blocks, out)
		case rschema.SetNestedBlock:
			collectUseStateForUnknownOnOptionalComputed(path, v.NestedObject.Attributes, v.NestedObject.Blocks, out)
		}
	}
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// hasFrameworkUseStateForUnknown reports whether the attribute carries a
// stock framework UseStateForUnknown. Compared via DeepEqual against
// canonical instances since the framework's modifier types are unexported.
func hasFrameworkUseStateForUnknown(a rschema.Attribute) bool {
	switch v := a.(type) {
	case rschema.StringAttribute:
		return containsModifier(v.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	case rschema.BoolAttribute:
		return containsModifier(v.PlanModifiers, boolplanmodifier.UseStateForUnknown())
	case rschema.Int64Attribute:
		return containsModifier(v.PlanModifiers, int64planmodifier.UseStateForUnknown())
	case rschema.Float64Attribute:
		return containsModifier(v.PlanModifiers, float64planmodifier.UseStateForUnknown())
	case rschema.ListAttribute:
		return containsModifier(v.PlanModifiers, listplanmodifier.UseStateForUnknown())
	case rschema.SetAttribute:
		return containsModifier(v.PlanModifiers, setplanmodifier.UseStateForUnknown())
	case rschema.MapAttribute:
		return containsModifier(v.PlanModifiers, mapplanmodifier.UseStateForUnknown())
	case rschema.ListNestedAttribute:
		return containsModifier(v.PlanModifiers, listplanmodifier.UseStateForUnknown())
	case rschema.SetNestedAttribute:
		return containsModifier(v.PlanModifiers, setplanmodifier.UseStateForUnknown())
	case rschema.MapNestedAttribute:
		return containsModifier(v.PlanModifiers, mapplanmodifier.UseStateForUnknown())
	case rschema.SingleNestedAttribute:
		return containsModifier(v.PlanModifiers, objectplanmodifier.UseStateForUnknown())
	}
	return false
}

func containsModifier[T any](mods []T, want T) bool {
	for _, m := range mods {
		if reflect.DeepEqual(m, want) {
			return true
		}
	}
	return false
}
