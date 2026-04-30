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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// useStateForUnknownAllowlist names (resource_type, dotted_attr_path) pairs
// where Optional+Computed+UseStateForUnknown is intentional sticky behavior.
// Every entry needs a reason — the default of RemoveOnConfigNullList is
// safer; allowlisting asserts the field has no "remove = clear" semantic.
var useStateForUnknownAllowlist = map[string]map[string]string{
	"archestra_agent_tool": {
		"mcp_server_id": "credential binding chosen at Create time; remove-from-config has no clear semantic — preserve prior state.",
	},
	"archestra_agent": {
		"built_in_agent_config.max_rounds": "nested in an AtomicObject JSONB block; the parent block governs the diff, not the leaf.",
	},
	"archestra_mcp_registry_catalog_item": {
		"client_secret_id":       "computed when the backend auto-creates a BYOS vault reference for an inline `oauth_config.client_secret`; sticky once issued so removing the inline secret doesn't accidentally drop the stored reference.", //nolint:gosec // schema attribute name, not a credential
		"local_config_secret_id": "computed when the backend auto-creates a BYOS vault reference for inline `local_config` env values; sticky once issued so removing the inline values doesn't accidentally drop the stored reference.",       //nolint:gosec // schema attribute name, not a credential
	},
	"archestra_mcp_server_installation": {
		"secret_id": "computed when the backend auto-creates a secret to hold inline `user_config_values` / `environment_values`; sticky once issued so removing the inline values doesn't accidentally drop the stored reference.", //nolint:gosec // schema attribute name, not a credential
	},
	"archestra_organization_settings": stickyOrgSettings(
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
		"default_llm_model",
		"default_llm_provider",
		"embedding_chat_api_key_id",
		"embedding_model",
		"favicon",
		"footer_text",
		"global_tool_policy",
		"icon_logo",
		"limit_cleanup_interval",
		"logo",
		"logo_dark",
		"mcp_oauth_access_token_lifetime_seconds",
		"og_description",
		"onboarding_complete",
		"reranker_chat_api_key_id",
		"reranker_model",
		"show_two_factor",
		"slim_chat_error_ui",
	),
}

// stickyOrgSettings builds an allowlist sub-map sharing the same
// "sticky-from-state" reason for every passed-in attribute name.
func stickyOrgSettings(names ...string) map[string]string {
	const reason = "documented sticky-from-state field on archestra_organization_settings — omitting from HCL preserves the backend value via the merge-patch (per resource MarkdownDescription)."
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
