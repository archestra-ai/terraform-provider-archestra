package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A sharing grant names its principal in one of three ways: subject_type with
// subject_id, team_id, or service_account_id. The last two exist so a grant
// can reference a team or service account managed in the same configuration
// (and, in Crossplane, through teamRef or serviceAccountRef). Every form is
// stored completely, so plans compare equal whichever form was written.

// permissionGrantNestedObject is one grant of a sharing set.
func permissionGrantNestedObject(actionsDescription string) schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Validators: []validator.Object{permissionGrantFormValidator{}},
		Attributes: map[string]schema.Attribute{
			"subject_type":       schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Principal type: `user`, `team`, `serviceAccount`, `role` or `organization`. Set with `subject_id`, or leave both out and set `team_id` or `service_account_id`.", Validators: []validator.String{stringvalidator.OneOf(permissionSubjectTypes...)}},
			"subject_id":         schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Principal ID; use * for the organization principal.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"team_id":            schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Team to grant access to, such as `archestra_team.example.id`. Replaces `subject_type` and `subject_id`.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"service_account_id": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Service account to grant access to, such as `archestra_service_account.example.id`. Replaces `subject_type` and `subject_id`.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"actions":            schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: actionsDescription, Validators: []validator.Set{setvalidator.SizeBetween(1, 6), setvalidator.ValueStringsAre(stringvalidator.OneOf(permissionActions...))}},
		},
	}
}

// permissionGrantValue is the stored form of a grant read from the API.
func permissionGrantValue(subjectType, subjectID string, actions types.Set) (types.Object, error) {
	teamID, serviceAccountID := types.StringNull(), types.StringNull()
	switch subjectType {
	case "team":
		teamID = types.StringValue(subjectID)
	case "serviceAccount":
		serviceAccountID = types.StringValue(subjectID)
	}
	value, d := types.ObjectValue(permissionGrantObjectType.AttrTypes, map[string]attr.Value{
		"subject_type": types.StringValue(subjectType), "subject_id": types.StringValue(subjectID),
		"team_id": teamID, "service_account_id": serviceAccountID, "actions": actions,
	})
	if d.HasError() {
		return value, fmt.Errorf("%s", d)
	}
	return value, nil
}

var permissionSubjectTypes = []string{"user", "team", "serviceAccount", "role", "organization"}

// completePermissionGrants plans each configured grant in full, so the plan
// matches what a read stores.
type completePermissionGrants struct{}

func (completePermissionGrants) Description(context.Context) string {
	return "Fills in the subject of each grant from team_id or service_account_id, and the reverse."
}

func (m completePermissionGrants) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (completePermissionGrants) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	grants := make([]attr.Value, 0, len(req.ConfigValue.Elements()))
	for _, element := range req.ConfigValue.Elements() {
		grant, ok := element.(types.Object)
		if !ok || grant.IsUnknown() || grant.IsNull() {
			return
		}
		values := grant.Attributes()
		subjectType, _ := values["subject_type"].(types.String)
		subjectID, _ := values["subject_id"].(types.String)
		teamID, _ := values["team_id"].(types.String)
		serviceAccountID, _ := values["service_account_id"].(types.String)
		switch {
		case !teamID.IsNull():
			subjectType, subjectID, serviceAccountID = types.StringValue("team"), teamID, types.StringNull()
		case !serviceAccountID.IsNull():
			subjectType, subjectID, teamID = types.StringValue("serviceAccount"), serviceAccountID, types.StringNull()
		default:
			teamID, serviceAccountID = permissionGrantSubjectAs(subjectType, subjectID, "team"), permissionGrantSubjectAs(subjectType, subjectID, "serviceAccount")
		}
		planned, d := types.ObjectValue(permissionGrantObjectType.AttrTypes, map[string]attr.Value{
			"subject_type": subjectType, "subject_id": subjectID, "team_id": teamID, "service_account_id": serviceAccountID, "actions": values["actions"],
		})
		resp.Diagnostics.Append(d...)
		grants = append(grants, planned)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	planned, d := types.SetValue(permissionGrantObjectType, grants)
	resp.Diagnostics.Append(d...)
	resp.PlanValue = planned
}

// permissionGrantSubjectAs is the subject ID when the subject has the given
// type, unknown while the type is unknown, and null otherwise.
func permissionGrantSubjectAs(subjectType, subjectID types.String, kind string) types.String {
	if subjectType.IsUnknown() {
		return types.StringUnknown()
	}
	if subjectType.ValueString() == kind {
		return subjectID
	}
	return types.StringNull()
}

// permissionGrantFormValidator requires exactly one way of naming the principal.
type permissionGrantFormValidator struct{}

func (permissionGrantFormValidator) Description(context.Context) string {
	return "Set subject_type with subject_id, or team_id, or service_account_id."
}

func (v permissionGrantFormValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (permissionGrantFormValidator) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	values := req.ConfigValue.Attributes()
	set := func(name string) bool {
		value, ok := values[name].(types.String)
		return ok && !value.IsNull()
	}
	forms := 0
	for _, name := range []string{"subject_id", "team_id", "service_account_id"} {
		if set(name) {
			forms++
		}
	}
	switch {
	case forms != 1:
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Grant Principal", "Set exactly one of subject_id (with subject_type), team_id or service_account_id.")
	case set("subject_id") != set("subject_type"):
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Grant Principal", "subject_type and subject_id are set together; with team_id or service_account_id, leave both out.")
	}
}
