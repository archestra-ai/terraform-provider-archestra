package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// roleAttrSpec covers the body wire fields shared by CreateRole and
// UpdateRole. `permission` is a Map<resource, action[]>; Map Kind
// sends the whole structure on any change, which matches the backend's
// full-replace semantics for the JSONB column.
//
// `description` is OmitOnNull because the backend zod is
// `.string().max(200).transform(trim).optional()` — no `.nullable()`,
// so sending `null` is rejected as "Invalid input: expected string,
// received null". Removing the attribute from HCL leaves the backend
// value untouched, matching the wire contract.
var roleAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar, OmitOnNull: true},
	{TFName: "permission", JSONName: "permission", Kind: Map},
}

func (r *RoleResource) AttrSpecs() []AttrSpec {
	return roleAttrSpec
}

func (r *RoleResource) APIShape() any {
	return client.GetRoleResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - organizationId: ownership metadata. The API key authenticates the
//     org; surfacing the field would create a phantom diff against the
//     resource's implicit scoping.
func (r *RoleResource) KnownIntentionallySkipped() []string {
	return []string{"organizationId"}
}

// validRoleActions enumerates the action values the backend permission
// map accepts. Driven by the generated client's enum so a backend bump
// flips the validator with the codegen instead of silent drift.
var validRoleActions = []string{
	string(client.CreateRoleJSONBodyPermissionAdmin),
	string(client.CreateRoleJSONBodyPermissionCancel),
	string(client.CreateRoleJSONBodyPermissionCreate),
	string(client.CreateRoleJSONBodyPermissionDelete),
	string(client.CreateRoleJSONBodyPermissionEnable),
	string(client.CreateRoleJSONBodyPermissionQuery),
	string(client.CreateRoleJSONBodyPermissionRead),
	string(client.CreateRoleJSONBodyPermissionTeamAdmin),
	string(client.CreateRoleJSONBodyPermissionUpdate),
}
