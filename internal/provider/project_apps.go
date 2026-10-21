package provider

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func projectAppIDsAttribute() schema.SetAttribute {
	return agentIDSetAttribute("Apps linked into the project; omit to preserve links, or set an empty set to unlink all. Only links to apps the caller can read are observed or changed.", 0)
}

func (r *ProjectResource) readApps(ctx context.Context, data *ProjectResourceModel, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Project ID", err.Error())
		return false
	}
	result, err := r.client.GetProjectAppsWithResponse(ctx, id)
	if err != nil {
		diags.AddError("Project Apps Read Failed", err.Error())
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("Project Apps Read Failed", fmt.Sprintf("API returned status %d; the project was readable, so its state is retained.", result.StatusCode()))
		return false
	}
	ids := make([]string, 0, len(*result.JSON200))
	for _, app := range *result.JSON200 {
		ids = append(ids, app.Id.String())
	}
	sort.Strings(ids)
	value, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	data.AppIDs = value
	return !diags.HasError()
}

// writeApps links and unlinks only the difference, so concurrent links to
// unmanaged projects are never touched and each call is idempotent.
func (r *ProjectResource) writeApps(ctx context.Context, data *ProjectResourceModel, prior types.Set, diags *diag.Diagnostics) bool {
	if data.AppIDs.IsNull() || data.AppIDs.IsUnknown() {
		return true
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Project ID", err.Error())
		return false
	}
	desired, current := map[string]bool{}, map[string]bool{}
	var values []string
	diags.Append(data.AppIDs.ElementsAs(ctx, &values, false)...)
	for _, value := range values {
		desired[value] = true
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		values = nil
		diags.Append(prior.ElementsAs(ctx, &values, false)...)
		for _, value := range values {
			current[value] = true
		}
	}
	if diags.HasError() {
		return false
	}
	for _, change := range sortedSetDifference(desired, current) {
		appID, err := uuid.Parse(change)
		if err != nil {
			diags.AddError("Invalid App ID", err.Error())
			return false
		}
		result, err := r.client.LinkProjectAppWithResponse(ctx, id, appID)
		if err != nil {
			diags.AddError("Project App Link Failed", err.Error())
			return false
		}
		if result.StatusCode() != http.StatusOK {
			diags.AddError("Project App Link Failed", fmt.Sprintf("Linking app %s returned status %d; the app must exist and be readable by the caller.", change, result.StatusCode()))
			return false
		}
	}
	for _, change := range sortedSetDifference(current, desired) {
		appID, err := uuid.Parse(change)
		if err != nil {
			diags.AddError("Invalid App ID", err.Error())
			return false
		}
		result, err := r.client.UnlinkProjectAppWithResponse(ctx, id, appID)
		if err != nil {
			diags.AddError("Project App Unlink Failed", err.Error())
			return false
		}
		// 404 means the link is already gone.
		if result.StatusCode() != http.StatusOK && result.StatusCode() != http.StatusNotFound {
			diags.AddError("Project App Unlink Failed", fmt.Sprintf("Unlinking app %s returned status %d.", change, result.StatusCode()))
			return false
		}
	}
	return true
}

func sortedSetDifference(left, right map[string]bool) []string {
	var result []string
	for value := range left {
		if !right[value] {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
