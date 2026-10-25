package provider

import (
	"context"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// teamAttrSpec covers the team body. `members` is a separate
// add/remove flow on the AddTeamMember/RemoveTeamMember endpoints — it never
// rides on the team Create/Update body — so it's not in the spec.
//
// `organization_id` and `created_by` are computed-only and excluded by the
// drift-check helper.
var teamAttrSpec = []AttrSpec{
	{TFName: "external_group_ids", Kind: Synthetic},
	{TFName: "roles", JSONName: "roles", Kind: Set, OmitOnNull: true},
	{TFName: "parent_id", JSONName: "parentId", Kind: Scalar},
	{TFName: "labels", JSONName: "labels", Kind: Set, OmitOnNull: true, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "members", Kind: Synthetic},
}

func (r *TeamResource) AttrSpecs() []AttrSpec { return teamAttrSpec }

func (r *TeamResource) APIShape() any { return client.GetTeamResponse{} }

// KnownIntentionallySkipped: createdAt/updatedAt are audit timestamps;
// could be exposed as Computed-only later if users ask. organizationId /
// createdBy are top-level computed-only fields already covered via the
// schema (organizationId↔organization_id, createdBy↔created_by — the
// snake-case roundtrip works directly).
func (r *TeamResource) KnownIntentionallySkipped() []string {
	return []string{"createdAt", "updatedAt"}
}

func (r *TeamResource) refreshCurrentTeam(ctx context.Context, data *TeamResourceModel, diags *diag.Diagnostics) {
	result, err := r.client.GetTeamWithResponse(ctx, data.ID.ValueString(), nil)
	if err != nil {
		diags.AddError("Team Refresh Failed", err.Error())
		return
	}
	if result.JSON200 == nil {
		diags.AddError("Team Refresh Failed", "The saved team could not be read after writing.")
		return
	}
	r.flattenCurrentTeam(ctx, data, result, diags)
}
func (r *TeamResource) flattenCurrentTeam(ctx context.Context, data *TeamResourceModel, result *client.GetTeamResponse, diags *diag.Diagnostics) {
	wire := result.JSON200
	data.ParentID = types.StringPointerValue(wire.ParentId)
	data.CreatedByServiceAccountID = types.StringNull()
	if wire.CreatedByServiceAccountId != nil {
		data.CreatedByServiceAccountID = types.StringValue(wire.CreatedByServiceAccountId.String())
	}
	data.MyRole = types.StringNull()
	if wire.MyRole != nil {
		data.MyRole = types.StringValue(string(*wire.MyRole))
	}
	roles := wire.Roles
	if roles == nil {
		roles = []string{}
	}
	set, ds := types.SetValueFrom(ctx, types.StringType, roles)
	diags.Append(ds...)
	data.Roles = set
	if wire.Labels != nil {
		data.Labels = flattenAgentLabels(data.Labels, *wire.Labels)
	} else {
		data.Labels = flattenAgentLabels(data.Labels, nil)
	}
	descendants := []TeamDescendantModel{}
	if wire.DescendantTeams != nil {
		for _, team := range *wire.DescendantTeams {
			descendants = append(descendants, TeamDescendantModel{ID: types.StringValue(team.Id), Name: types.StringValue(team.Name)})
		}
	}
	value, ds := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "name": types.StringType}}, descendants)
	diags.Append(ds...)
	data.DescendantTeams = value

}
