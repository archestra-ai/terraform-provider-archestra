package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func teamBindingAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"external_group_ids": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, MarkdownDescription: "Complete SSO group selection for enterprise Team Sync; omit to preserve or set an empty set to clear.", PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}, Validators: []validator.Set{setvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))}},
		"vault_folder":       schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Team Vault folder path; omit to preserve or set an empty string to remove the binding, requiring Readonly Vault.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
	}
}

type teamGroupMapping struct{ ID, Identifier string }

func (r *TeamResource) externalGroupMappings(ctx context.Context, id string, unmanaged bool, diags *diag.Diagnostics) ([]teamGroupMapping, bool) {
	response, err := r.client.GetTeamExternalGroupsWithResponse(ctx, id)
	if err != nil {
		diags.AddError("Team groups read failed", err.Error())
		return nil, false
	}
	// Unconfigured licensed settings must not prevent ordinary team management.
	if response.StatusCode() == 403 && unmanaged {
		return nil, false
	}
	if response.JSON200 == nil {
		diags.AddError("Team groups read failed", fmt.Sprintf("API returned status %d; existing group state was retained.", response.StatusCode()))
		return nil, false
	}
	seen := map[string]bool{}
	rows := make([]teamGroupMapping, 0, len(*response.JSON200))
	for _, row := range *response.JSON200 {
		if row.TeamId != id || seen[row.GroupIdentifier] {
			diags.AddError("Ambiguous team groups", "The API returned a foreign or duplicate group mapping; no membership changes were made.")
			return nil, false
		}
		seen[row.GroupIdentifier] = true
		rows = append(rows, teamGroupMapping{ID: row.Id, Identifier: row.GroupIdentifier})
	}
	return rows, true
}

func (r *TeamResource) readBindings(ctx context.Context, data *TeamResourceModel, diags *diag.Diagnostics) {
	rows, readable := r.externalGroupMappings(ctx, data.ID.ValueString(), data.ExternalGroupIDs.IsNull() || data.ExternalGroupIDs.IsUnknown(), diags)
	if diags.HasError() {
		return
	}
	if readable {
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.Identifier)
		}
		data.ExternalGroupIDs = agentSet(ctx, ids, diags)
	} else {
		data.ExternalGroupIDs = types.SetNull(types.StringType)
	}
	folder, err := r.client.GetTeamVaultFolderWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		diags.AddError("Team Vault folder read failed", err.Error())
		return
	}
	if folder.StatusCode() == 403 && (data.VaultFolder.IsNull() || data.VaultFolder.IsUnknown()) {
		data.VaultFolder = types.StringNull()
		return
	}
	if folder.StatusCode() == 200 && strings.TrimSpace(string(folder.Body)) == "null" {
		data.VaultFolder = types.StringValue("")
		return
	}
	if folder.JSON200 == nil {
		diags.AddError("Team Vault folder read failed", fmt.Sprintf("API returned status %d; existing folder state was retained.", folder.StatusCode()))
		return
	}
	if folder.JSON200.TeamId != data.ID.ValueString() {
		diags.AddError("Unexpected Vault folder scope", "The API returned another team's folder.")
		return
	}
	data.VaultFolder = types.StringValue(folder.JSON200.VaultPath)
}

func (r *TeamResource) writeBindings(ctx context.Context, plan, prior *TeamResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	if agentConfigured(ctx, config, "external_group_ids", diags) && !plan.ExternalGroupIDs.Equal(prior.ExternalGroupIDs) {
		r.replaceExternalGroups(ctx, plan, diags)
	}
	var configured types.String
	diags.Append(config.GetAttribute(ctx, path.Root("vault_folder"), &configured)...)
	if diags.HasError() || configured.IsNull() || configured.IsUnknown() || plan.VaultFolder.Equal(prior.VaultFolder) {
		return
	}
	if configured.ValueString() == "" {
		result, err := r.client.DeleteTeamVaultFolderWithResponse(ctx, plan.ID.ValueString())
		if err != nil {
			diags.AddError("Team Vault folder removal failed", err.Error())
		} else if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
			diags.AddError("Team Vault folder removal failed", fmt.Sprintf("API returned status %d.", result.StatusCode()))
		}
		return
	}
	result, err := r.client.SetTeamVaultFolderWithResponse(ctx, plan.ID.ValueString(), client.SetTeamVaultFolderJSONRequestBody{VaultPath: configured.ValueString()})
	if err != nil {
		diags.AddError("Team Vault folder update failed", err.Error())
	} else if result.JSON200 == nil {
		diags.AddError("Team Vault folder update failed", fmt.Sprintf("API returned status %d.", result.StatusCode()))
	}
}

func (r *TeamResource) replaceExternalGroups(ctx context.Context, data *TeamResourceModel, diags *diag.Diagnostics) {
	var desired []string
	diags.Append(data.ExternalGroupIDs.ElementsAs(ctx, &desired, false)...)
	if diags.HasError() {
		return
	}
	rows, readable := r.externalGroupMappings(ctx, data.ID.ValueString(), false, diags)
	if !readable {
		return
	}
	wanted, existing := map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		existing[row.Identifier] = true
	}
	for _, id := range desired {
		wanted[id] = true
		if existing[id] {
			continue
		}
		result, err := r.client.AddTeamExternalGroupWithResponse(ctx, data.ID.ValueString(), client.AddTeamExternalGroupJSONRequestBody{GroupIdentifier: id})
		if err != nil {
			diags.AddError("Team group assignment failed", err.Error())
			return
		}
		if result.JSON200 == nil || result.JSON200.TeamId != data.ID.ValueString() || result.JSON200.GroupIdentifier != id {
			diags.AddError("Team group assignment rejected", fmt.Sprintf("API returned status %d; extra existing mappings were retained.", result.StatusCode()))
			return
		}
	}
	for _, row := range rows {
		if wanted[row.Identifier] {
			continue
		}
		result, err := r.client.RemoveTeamExternalGroupWithResponse(ctx, data.ID.ValueString(), row.ID)
		if err != nil {
			diags.AddError("Team group removal failed", err.Error())
			return
		}
		if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
			diags.AddError("Team group removal failed", fmt.Sprintf("API returned status %d.", result.StatusCode()))
			return
		}
	}
	confirmed, ok := r.externalGroupMappings(ctx, data.ID.ValueString(), false, diags)
	if !ok {
		return
	}
	ids := make([]string, 0, len(confirmed))
	for _, row := range confirmed {
		ids = append(ids, row.Identifier)
	}
	if !agentSet(ctx, ids, diags).Equal(data.ExternalGroupIDs) {
		diags.AddError("Incomplete team group replacement", "The API did not retain the requested complete group set.")
	}
}

func (r *TeamResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}

func (r *TeamResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var folder types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("vault_folder"), &folder)...)
	value := folder.ValueString()
	if strings.Contains(value, "..") || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		resp.Diagnostics.AddAttributeError(path.Root("vault_folder"), "Invalid Vault folder path", "Use a relative Vault path without '..' or leading/trailing slashes, or an empty string to remove the binding.")
	}
}

func knownTeamState(data TeamResourceModel) TeamResourceModel {
	if data.ExternalGroupIDs.IsUnknown() {
		data.ExternalGroupIDs = types.SetNull(types.StringType)
	}
	if data.VaultFolder.IsUnknown() {
		data.VaultFolder = types.StringNull()
	}
	if data.Roles.IsUnknown() {
		data.Roles = types.SetNull(types.StringType)
	}
	if data.DescendantTeams.IsUnknown() {
		data.DescendantTeams = types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "name": types.StringType}})
	}
	if data.MyRole.IsUnknown() {
		data.MyRole = types.StringNull()
	}
	if data.CreatedByServiceAccountID.IsUnknown() {
		data.CreatedByServiceAccountID = types.StringNull()
	}
	return data
}
