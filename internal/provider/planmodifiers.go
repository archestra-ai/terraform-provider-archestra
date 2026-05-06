package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// RemoveOnConfigNullList plans null when the attribute is null in config so
// the merge-patch clears the field on the backend. Use instead of
// UseStateForUnknown on Optional+Computed lists where "remove from config"
// means "clear it." For sticky semantics, allowlist UseStateForUnknown in
// planmodifier_audit_test.go with a reason.
func RemoveOnConfigNullList() planmodifier.List { return removeOnConfigNullList{} }

type removeOnConfigNullList struct{}

func (m removeOnConfigNullList) Description(_ context.Context) string {
	return "When the attribute is null in configuration, plan null so the backend clears it on next apply."
}

func (m removeOnConfigNullList) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m removeOnConfigNullList) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	resp.PlanValue = types.ListNull(req.ConfigValue.ElementType(ctx))
}

// EmptyListOnConfigNull plans an empty list when the attribute is null in
// config. Use on backend fields where the wire schema is `array.optional()`
// rather than `array.nullable()`: sending JSON null produces 400 ("expected
// array, received null"), but sending `[]` is accepted and clears the field.
// HCL `attr = []` and omit-attr collapse to the same wire payload, which is
// the right semantic for that class of backend.
func EmptyListOnConfigNull() planmodifier.List { return emptyListOnConfigNull{} }

type emptyListOnConfigNull struct{}

func (m emptyListOnConfigNull) Description(_ context.Context) string {
	return "When the attribute is null in configuration, plan an empty list so the backend clears it without rejecting `null`."
}

func (m emptyListOnConfigNull) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m emptyListOnConfigNull) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	empty, diags := types.ListValue(req.ConfigValue.ElementType(ctx), []attr.Value{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.PlanValue = empty
}
