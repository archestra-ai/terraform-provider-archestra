package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A limit names what it applies to with entity_type and entity_id, or with
// one target attribute such as team_id. The target attributes let a limit
// reference an object managed in the same configuration (and, in Crossplane,
// use teamRef or virtualKeyRef). Every form is stored completely: entity_type
// and entity_id always, and the target attribute matching entity_type.

var _ resource.ResourceWithModifyPlan = &LimitResource{}

// limitTargets maps each target attribute to the entity type it sets.
var limitTargets = []struct{ attribute, entityType, description string }{
	{"team_id", "team", "Team the limit applies to, such as `archestra_team.example.id`."},
	{"virtual_key_id", "virtual_key", "Virtual key the limit applies to, such as `archestra_virtual_key.example.id`. A key's cost budget is its `spend_cap`; use this for request and token quotas."},
	{"agent_id", "agent", "Agent the limit applies to, such as `archestra_agent.example.id`."},
	{"environment_id", "environment", "Environment the limit applies to, such as `archestra_environment.example.id`."},
	{"llm_oauth_client_id", "llm_oauth_client", "OAuth client the limit applies to, such as `archestra_oauth_client.example.id`."},
}

func limitTargetAttributes() map[string]schema.Attribute {
	attributes := map[string]schema.Attribute{}
	for _, target := range limitTargets {
		attributes[target.attribute] = schema.StringAttribute{
			Optional: true, Computed: true,
			Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			MarkdownDescription: target.description + " Replaces `entity_type` and `entity_id`. Changing it replaces the limit.",
		}
	}
	return attributes
}

// limitConfiguredTarget is the entity a configuration names: from a target
// attribute when one is set, otherwise from entity_type and entity_id.
func limitConfiguredTarget(entityType, entityID types.String, targets map[string]types.String) (types.String, types.String) {
	for _, target := range limitTargets {
		if value := targets[target.attribute]; !value.IsNull() {
			return types.StringValue(target.entityType), value
		}
	}
	return entityType, entityID
}

// setLimitTargets stores the target attribute matching entity_type.
func (data *LimitResourceModel) setLimitTargets() {
	for _, target := range limitTargets {
		value := types.StringNull()
		switch {
		case data.EntityType.IsUnknown():
			value = types.StringUnknown()
		case data.EntityType.ValueString() == target.entityType:
			value = data.EntityID
		}
		data.setLimitTarget(target.attribute, value)
	}
}

func (data *LimitResourceModel) setLimitTarget(attribute string, value types.String) {
	switch attribute {
	case "team_id":
		data.TeamID = value
	case "virtual_key_id":
		data.VirtualKeyID = value
	case "agent_id":
		data.AgentID = value
	case "environment_id":
		data.EnvironmentID = value
	case "llm_oauth_client_id":
		data.LLMOAuthClientID = value
	}
}

func (data *LimitResourceModel) limitTargetValue(attribute string) types.String {
	switch attribute {
	case "team_id":
		return data.TeamID
	case "virtual_key_id":
		return data.VirtualKeyID
	case "agent_id":
		return data.AgentID
	case "environment_id":
		return data.EnvironmentID
	case "llm_oauth_client_id":
		return data.LLMOAuthClientID
	}
	return types.StringNull()
}

// ModifyPlan plans the entity from the configured form, so entity_type,
// entity_id and the target attributes are known and consistent, and replaces
// the limit when the entity changes.
func (r *LimitResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var config, plan LimitResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	targets := map[string]types.String{}
	for _, target := range limitTargets {
		targets[target.attribute] = config.limitTargetValue(target.attribute)
	}
	plan.EntityType, plan.EntityID = limitConfiguredTarget(config.EntityType, config.EntityID, targets)
	plan.setLimitTargets()
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)

	if req.State.Raw.IsNull() {
		return
	}
	var state LimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, name := range []string{"entity_type", "entity_id"} {
		planned, prior := plan.EntityType, state.EntityType
		if name == "entity_id" {
			planned, prior = plan.EntityID, state.EntityID
		}
		if planned.IsUnknown() || !planned.Equal(prior) {
			resp.RequiresReplace = append(resp.RequiresReplace, path.Root(name))
		}
	}
}

// validateLimitTarget requires exactly one way of naming the entity, and
// returns the configured entity type (null when unknown or invalid).
func validateLimitTarget(data *LimitResourceModel, diags interface {
	AddAttributeError(path.Path, string, string)
}) types.String {
	named := 0
	for _, target := range limitTargets {
		if !data.limitTargetValue(target.attribute).IsNull() {
			named++
		}
	}
	generic := !data.EntityID.IsNull() || !data.EntityType.IsNull()
	switch {
	case named > 1 || (named == 1 && generic):
		diags.AddAttributeError(path.Root("entity_id"), "Invalid Limit Target", "Set one of team_id, virtual_key_id, agent_id, environment_id or llm_oauth_client_id, or set entity_type with entity_id.")
		return types.StringNull()
	case named == 0 && (data.EntityID.IsNull() || data.EntityType.IsNull()):
		diags.AddAttributeError(path.Root("entity_id"), "Missing Limit Target", "Set entity_type with entity_id, or one of team_id, virtual_key_id, agent_id, environment_id or llm_oauth_client_id.")
		return types.StringNull()
	}
	targets := map[string]types.String{}
	for _, target := range limitTargets {
		targets[target.attribute] = data.limitTargetValue(target.attribute)
	}
	entityType, _ := limitConfiguredTarget(data.EntityType, data.EntityID, targets)
	return entityType
}
