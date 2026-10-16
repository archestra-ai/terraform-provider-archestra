package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func initialGrantsAttribute() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true, Computed: true,
		MarkdownDescription: "Additional direct sharing grants on this object. Applied atomically on creation and reconciled in place thereafter. Omit to preserve current grants; set [] to clear additional grants. The owner's existing grant is preserved automatically; inherited access and ownership remain managed by Archestra.",
		PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
		Validators:          []validator.Set{setvalidator.SizeAtMost(200)},
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"subject_type": schema.StringAttribute{Required: true, MarkdownDescription: "Principal type.", Validators: []validator.String{stringvalidator.OneOf("user", "team", "serviceAccount", "role", "organization")}},
			"subject_id":   schema.StringAttribute{Required: true, MarkdownDescription: "Principal ID; use * for the organization principal.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"actions":      schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Actions granted to the principal.", Validators: []validator.Set{setvalidator.SizeBetween(1, 6), setvalidator.ValueStringsAre(stringvalidator.OneOf(permissionActions...))}},
		}},
	}
}

var initialGrantsAttrSpec = AttrSpec{TFName: "permissions", JSONName: "initialGrants", Kind: Set, OmitOnNull: true, CreateOnly: true, Children: []AttrSpec{
	{TFName: "subject_type", JSONName: "subject_type", Kind: Scalar},
	{TFName: "subject_id", JSONName: "subject_id", Kind: Scalar},
	{TFName: "actions", JSONName: "actions", Kind: Set},
}, Encoder: encodeInitialGrants}

func encodeInitialGrants(value any) any {
	values, ok := value.([]any)
	if !ok {
		return value
	}
	result := make([]any, 0, len(values))
	for _, value := range values {
		grant, ok := value.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, map[string]any{"subject": map[string]any{"type": grant["subject_type"], "id": grant["subject_id"]}, "actions": grant["actions"]})
	}
	return result
}

// The create response does not echo the creation-only input.
func resolveInitialGrants(value *types.Set) {
	if value.IsUnknown() || value.IsNull() {
		*value = types.SetNull(permissionGrantObjectType)
	}
}

func nonAtomicPermissionsAttribute() schema.SetNestedAttribute {
	value := initialGrantsAttribute()
	value.MarkdownDescription = "Additional direct sharing grants. Applied after the parent identity exists, then reconciled in place. Omit to preserve grants; [] clears additional grants while retaining the owner's existing access."
	return value
}
