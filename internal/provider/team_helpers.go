package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// teamAttrSpec covers the team body. `members` is a separate
// add/remove flow on the AddTeamMember/RemoveTeamMember endpoints — it never
// rides on the team Create/Update body — so it's not in the spec.
//
// `organization_id` and `created_by` are computed-only and excluded by the
// drift-check helper.
var teamAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "convert_tool_results_to_toon", JSONName: "convertToolResultsToToon", Kind: Scalar},
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
