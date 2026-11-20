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
		PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown(), completePermissionGrants{}},
		Validators:          []validator.Set{setvalidator.SizeAtMost(200)},
		NestedObject:        permissionGrantNestedObject("Actions granted to the principal."),
	}
}

var initialGrantsAttrSpec = AttrSpec{TFName: "permissions", JSONName: "initialGrants", Kind: Set, OmitOnNull: true, CreateOnly: true, Children: []AttrSpec{
	{TFName: "subject_type", JSONName: "subject_type", Kind: Scalar},
	{TFName: "subject_id", JSONName: "subject_id", Kind: Scalar},
	{TFName: "team_id", JSONName: "team_id", Kind: Scalar},
	{TFName: "service_account_id", JSONName: "service_account_id", Kind: Scalar},
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

// Environment-only actions, introduced in 1.5.0-rc.37. A grant of read plus
// some of them deploys only those kinds; use deploys every kind.
var environmentDeployActions = []string{"deploy-agent", "deploy-mcp-gateway", "deploy-mcp-registry", "deploy-skill", "deploy-app", "deploy-knowledge-source"}

func withEnvironmentDeployActions(value schema.SetNestedAttribute) schema.SetNestedAttribute {
	actions, _ := value.NestedObject.Attributes["actions"].(schema.SetAttribute)
	actions.Validators = []validator.Set{setvalidator.SizeBetween(1, 8), setvalidator.ValueStringsAre(stringvalidator.OneOf(append(append([]string{}, permissionActions...), environmentDeployActions...)...))}
	value.NestedObject.Attributes["actions"] = actions
	return value
}
