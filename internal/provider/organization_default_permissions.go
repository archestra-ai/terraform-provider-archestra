package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var defaultPermissionSlots = map[string]string{
	"agents": "agent", "mcp_gateways": "mcpGateway", "mcp_registry": "mcpRegistry", "skills": "skill", "apps": "app", "models": "llmModel", "projects": "project", "conversations": "conversation", "agent_runs": "agentRun", "plugins": "plugin", "knowledge_bases": "knowledgeBase", "knowledge_connectors": "knowledgeConnector", "knowledge_files": "knowledgeFile", "virtual_keys": "llmVirtualKey", "model_providers": "llmProviderApiKey", "remote_agents": "externalAgent", "oauth_clients": "oauthClient", "environments": "environment", "service_accounts": "serviceAccount", "credentials": "credential",
}
var defaultPermissionsType = func() types.ObjectType {
	attributes := map[string]attr.Type{}
	for name := range defaultPermissionSlots {
		attributes[name] = types.SetType{ElemType: permissionGrantObjectType}
	}
	return types.ObjectType{AttrTypes: attributes}
}()

func defaultPermissionsAttribute() schema.SingleNestedAttribute {
	attributes := map[string]schema.Attribute{}
	for name := range defaultPermissionSlots {
		value := initialGrantsAttribute()
		value.MarkdownDescription = "Organization-wide grants for every object of this type. Omit to preserve the current policy; [] clears its direct grants. Per-object sharing remains on the object."
		actions, _ := value.NestedObject.Attributes["actions"].(schema.SetAttribute)
		actions.MarkdownDescription = "Actions granted to the principal; keep manage-permissions on at least one grant in each policy."
		value.NestedObject.Attributes["actions"] = actions
		attributes[name] = value
	}
	return schema.SingleNestedAttribute{Optional: true, Computed: true, Attributes: attributes, PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Default access policies from organization Settings. Manage each object type independently: omit a field to leave its policy unmanaged, or set [] to clear that type's direct organization-wide grants. Changing these policies affects existing and future objects. Organization-settings deletion leaves them intact. The API rejects any change, including [], that leaves a policy without a grant that includes manage-permissions."}
}

func (r *OrganizationSettingsResource) readDefaultPermissions(ctx context.Context, data *OrganizationSettingsResourceModel, diags *diag.Diagnostics) {
	previous := data.DefaultPermissions.Attributes()
	values := map[string]attr.Value{}
	helper := ResourcePermissionPolicyResource{client: r.client}
	for name, kind := range defaultPermissionSlots {
		prior := types.SetNull(permissionGrantObjectType)
		if old, ok := previous[name].(types.Set); ok {
			prior = old
		}
		state := parentPermissionState(kind, "*")
		var readDiags diag.Diagnostics
		policy, status := helper.readPolicy(ctx, &state, &readDiags)
		if status == 403 && (prior.IsNull() || prior.IsUnknown()) {
			values[name] = types.SetNull(permissionGrantObjectType)
			continue
		}
		diags.Append(readDiags...)
		if policy == nil {
			if status == 404 {
				diags.AddError("Default permission policy unavailable", fmt.Sprintf("The %s organization-wide policy could not be read.", name))
			}
			return
		}
		values[name] = parentPermissionGrants(ctx, &state, policy, diags)
		if diags.HasError() {
			return
		}
	}
	value, d := types.ObjectValue(defaultPermissionsType.AttrTypes, values)
	diags.Append(d...)
	if !diags.HasError() {
		data.DefaultPermissions = value
	}
}

func (r *OrganizationSettingsResource) applyDefaultPermissions(ctx context.Context, plan, prior tftypes.Value, diags *diag.Diagnostics) {
	desired, previous := lookupOrNull(plan, "default_permissions"), lookupOrNull(prior, "default_permissions")
	if !desired.IsKnown() || desired.IsNull() || desired.Equal(previous) {
		return
	}
	var fields map[string]tftypes.Value
	if err := desired.As(&fields); err != nil {
		diags.AddError("Invalid default permissions", err.Error())
		return
	}
	helper := ResourcePermissionPolicyResource{client: r.client}
	for name, kind := range defaultPermissionSlots {
		want, old := fields[name], lookupOrNull(previous, name)
		if !want.IsKnown() || want.IsNull() || want.Equal(old) {
			continue
		}
		desiredValue, d := types.SetType{ElemType: permissionGrantObjectType}.ValueFromTerraform(ctx, want)
		if d != nil {
			diags.AddError("Invalid organization defaults", d.Error())
			return
		}
		wanted, ok := desiredValue.(types.Set)
		if !ok || diags.HasError() {
			if !diags.HasError() {
				diags.AddError("Invalid default permissions", "Expected a set of sharing grants.")
			}
			return
		}
		state := parentPermissionState(kind, "*")
		current, status := helper.readPolicy(ctx, &state, diags)
		if current == nil {
			if status == 404 {
				diags.AddError("Default permission policy unavailable", "Refresh the organization-wide policy before applying.")
			}
			return
		}
		observed := parentPermissionGrants(ctx, &state, current, diags)
		if diags.HasError() {
			return
		}
		if old.IsKnown() && !old.IsNull() {
			oldValue, d := types.SetType{ElemType: permissionGrantObjectType}.ValueFromTerraform(ctx, old)
			if d != nil {
				diags.AddError("Invalid organization defaults", d.Error())
				return
			}
			previousSet, ok := oldValue.(types.Set)
			if !ok || diags.HasError() {
				return
			}
			if !observed.Equal(previousSet) {
				diags.AddError("Default permission revision conflict", "An organization-wide policy changed after planning. Refresh and review it before applying again.")
				return
			}
		}
		if observed.Equal(wanted) {
			continue
		}
		grants := permissionGrantsFromPlan(ctx, wanted, diags)
		if diags.HasError() {
			return
		}
		updated := helper.writePolicy(ctx, &state, current.Revision, grants, diags)
		if updated == nil {
			return
		}
		actual := parentPermissionGrants(ctx, &state, updated, diags)
		if !diags.HasError() && !actual.Equal(wanted) {
			diags.AddError("Default permission verification failed", "The API did not persist the requested organization-wide sharing grants.")
			return
		}
	}
}
