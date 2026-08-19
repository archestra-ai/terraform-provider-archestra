package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// scheduleTriggerAttrSpec covers the body wire fields shared by
// CreateScheduleTrigger and UpdateScheduleTrigger. Create's `name`,
// `agentId`, `messageTemplate`, `cronExpression`, and `timezone` are
// required; Update declares every field optional, so MergePatch's
// only-changed-fields output is valid for both.
var scheduleTriggerAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "agent_id", JSONName: "agentId", Kind: Scalar},
	{TFName: "message_template", JSONName: "messageTemplate", Kind: Scalar},
	{TFName: "cron_expression", JSONName: "cronExpression", Kind: Scalar},
	{TFName: "timezone", JSONName: "timezone", Kind: Scalar},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
}

func (r *ScheduleTriggerResource) AttrSpecs() []AttrSpec {
	return scheduleTriggerAttrSpec
}

func (r *ScheduleTriggerResource) APIShape() any {
	return client.GetScheduleTriggerResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - organizationId: ownership metadata. Implicit in the API key.
//   - actor / agent: denormalized convenience metadata. `agent_id` and
//     `actor_user_id` already carry the foreign keys; users can look
//     up the full agent record via existing resources/data sources.
func (r *ScheduleTriggerResource) KnownIntentionallySkipped() []string {
	return []string{"organizationId", "actor", "agent"}
}
