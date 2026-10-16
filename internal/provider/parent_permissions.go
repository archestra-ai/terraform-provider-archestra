package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Permission responses return all direct grants, including unavailable recipients.
// Keep their stable subject IDs; inherited grants and ownership are never replaced.
func parentPermissionState(resourceType, id string) ResourcePermissionPolicyResourceModel {
	return ResourcePermissionPolicyResourceModel{ResourceType: types.StringValue(resourceType), ResourceID: types.StringValue(id)}
}

func parentPermissionGrants(ctx context.Context, state *ResourcePermissionPolicyResourceModel, policy *permissionPolicyWire, diags *diag.Diagnostics) types.Set {
	if policy.Resource != state.ResourceType.ValueString() || policy.Scope != state.ResourceID.ValueString() {
		diags.AddError("Unexpected permission target", "The permissions response did not identify the requested parent.")
		return types.SetNull(permissionGrantObjectType)
	}
	owner, ok := parentPermissionOwner(policy, diags)
	if !ok {
		return types.SetNull(permissionGrantObjectType)
	}
	observed := *policy
	observed.Grants = make([]permissionGrantWire, 0, len(policy.Grants))
	for _, grant := range policy.Grants {
		if !isParentOwnerGrant(grant, owner) {
			observed.Grants = append(observed.Grants, grant)
		}
	}
	setPermissionPolicyState(ctx, state, &observed, diags)
	return state.Grants
}

func readParentPermissions(ctx context.Context, api *client.ClientWithResponses, resourceType, id string, grants *types.Set, diags *diag.Diagnostics) {
	state := parentPermissionState(resourceType, id)
	helper := ResourcePermissionPolicyResource{client: api}
	var readDiags diag.Diagnostics
	policy, status := helper.readPolicy(ctx, &state, &readDiags)
	// A reader can manage ordinary metadata without managing inaccessible sharing.
	if status == 403 && (grants.IsNull() || grants.IsUnknown()) {
		*grants = types.SetNull(permissionGrantObjectType)
		return
	}
	diags.Append(readDiags...)
	if policy == nil {
		if status == 404 {
			diags.AddError("Parent permission policy unavailable", "The parent exists, but its permission policy could not be read.")
		}
		return
	}
	observed := parentPermissionGrants(ctx, &state, policy, diags)
	if !diags.HasError() {
		*grants = observed
	}
}

// Check the observed policy before any parent mutation, then use that revision
// for the final permission write. A concurrent edit requires a fresh plan.
func beginParentPermissionUpdate(ctx context.Context, api *client.ClientWithResponses, resourceType, idField string, req resource.UpdateRequest, resp *resource.UpdateResponse) func() {
	var desired, prior types.Set
	var id types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("permissions"), &desired)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("permissions"), &prior)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(idField), &id)...)
	noop := func() {}
	if resp.Diagnostics.HasError() || desired.IsNull() || desired.IsUnknown() || desired.Equal(prior) {
		return noop
	}
	state := parentPermissionState(resourceType, id.ValueString())
	helper := ResourcePermissionPolicyResource{client: api}
	current, status := helper.readPolicy(ctx, &state, &resp.Diagnostics)
	if current == nil {
		if status == 404 {
			resp.Diagnostics.AddError("Parent permission policy unavailable", "Refresh the parent before changing its permissions.")
		}
		return noop
	}
	observed := parentPermissionGrants(ctx, &state, current, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return noop
	}
	if !prior.IsNull() && !prior.IsUnknown() && !observed.Equal(prior) {
		resp.Diagnostics.AddError("Permission revision conflict", "The parent's direct grants changed after planning. Refresh and review the new grants before applying again.")
		return noop
	}
	grants := parentPermissionRequest(ctx, current, desired, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return noop
	}
	return func() {
		if resp.Diagnostics.HasError() {
			return
		}
		updated := helper.writePolicy(ctx, &state, current.Revision, grants, &resp.Diagnostics)
		if updated == nil {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("permissions"), observed)...)
			return
		}
		actual := parentPermissionGrants(ctx, &state, updated, &resp.Diagnostics)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("permissions"), actual)...)
		if !resp.Diagnostics.HasError() && !actual.Equal(desired) {
			resp.Diagnostics.AddError("Permission reconciliation failed", fmt.Sprintf("The %s parent did not persist the requested direct grants.", resourceType))
		}
	}
}

// Creation defaults can include a creator grant. Observe them before returning
// so a subsequent import sees the same persisted sharing configuration.
func finishParentCreate(ctx context.Context, api *client.ClientWithResponses, resourceType, idField string, resp *resource.CreateResponse) {
	if resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		return
	}
	var id types.String
	var desired types.Set
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root(idField), &id)...)
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("permissions"), &desired)...)
	if resp.Diagnostics.HasError() || id.IsNull() || id.IsUnknown() || id.ValueString() == "" {
		return
	}
	state := parentPermissionState(resourceType, id.ValueString())
	helper := ResourcePermissionPolicyResource{client: api}
	var readDiags diag.Diagnostics
	current, status := helper.readPolicy(ctx, &state, &readDiags)
	if status == 403 && (desired.IsNull() || desired.IsUnknown()) {
		return
	}
	resp.Diagnostics.Append(readDiags...)
	if current == nil {
		if status == 404 {
			resp.Diagnostics.AddError("Parent permission policy unavailable", "The created parent has no readable permission policy.")
		}
		return
	}
	observed := parentPermissionGrants(ctx, &state, current, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// Creation always gives its creator access. For an organization-owned
	// object this is an ordinary grant; reconcile explicitly configured sharing.
	if !desired.IsNull() && !desired.IsUnknown() && !desired.Equal(observed) {
		grants := parentPermissionRequest(ctx, current, desired, &resp.Diagnostics)
		if !resp.Diagnostics.HasError() {
			if updated := helper.writePolicy(ctx, &state, current.Revision, grants, &resp.Diagnostics); updated != nil {
				observed = parentPermissionGrants(ctx, &state, updated, &resp.Diagnostics)
			}
		}
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("permissions"), observed)...)
}

func parentPermissionRequest(ctx context.Context, current *permissionPolicyWire, desired types.Set, diags *diag.Diagnostics) []permissionGrantWire {
	grants := permissionGrantsFromPlan(ctx, desired, diags)
	owner, ok := parentPermissionOwner(current, diags)
	if !ok {
		return nil
	}
	for _, grant := range grants {
		if isParentOwnerGrant(grant, owner) {
			diags.AddAttributeError(path.Root("permissions"), "Owner grant is managed by ownership", "Declare additional sharing recipients here; the owner's existing access is preserved automatically.")
			return nil
		}
	}
	for _, grant := range current.Grants {
		if isParentOwnerGrant(grant, owner) {
			grants = append(grants, grant)
		}
	}
	return grants
}

type parentOwnerSubject struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// ownerId is a human ownership identity. Service-account creation is not ownership.
func parentPermissionOwner(policy *permissionPolicyWire, diags *diag.Diagnostics) (*parentOwnerSubject, bool) {
	if policy.OwnerID == nil {
		return nil, true
	}
	if *policy.OwnerID == "" {
		diags.AddError("Invalid permission owner", "The API returned an empty ownership identifier.")
		return nil, false
	}
	return &parentOwnerSubject{Type: "user", ID: *policy.OwnerID}, true
}

func isParentOwnerGrant(grant permissionGrantWire, owner *parentOwnerSubject) bool {
	return owner != nil && grant.Subject.Type == owner.Type && grant.Subject.ID == owner.ID
}
