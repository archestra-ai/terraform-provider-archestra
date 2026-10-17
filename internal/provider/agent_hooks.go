package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type agentInlineHook struct {
	Event        types.String `tfsdk:"event"`
	FileName     types.String `tfsdk:"file_name"`
	Content      types.String `tfsdk:"content"`
	Requirements types.List   `tfsdk:"requirements"`
	Enabled      types.Bool   `tfsdk:"enabled"`
}
type agentHookWire struct {
	ID           string   `json:"id"`
	AgentID      string   `json:"agentId"`
	Event        string   `json:"event"`
	FileName     string   `json:"fileName"`
	Content      string   `json:"content"`
	Requirements []string `json:"requirements"`
	Enabled      bool     `json:"enabled"`
}

var agentHookType = types.ObjectType{AttrTypes: map[string]attr.Type{"event": types.StringType, "file_name": types.StringType, "content": types.StringType, "requirements": types.ListType{ElemType: types.StringType}, "enabled": types.BoolType}}

func agentHooksAttribute() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{Optional: true, Computed: true, MarkdownDescription: "Complete event hook configuration; omit to preserve hooks or set an empty set to remove them.", PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"event":        schema.StringAttribute{Required: true, MarkdownDescription: "Event that runs the hook.", Validators: []validator.String{stringvalidator.OneOf("session_start", "pre_tool_use", "post_tool_use")}},
		"file_name":    schema.StringAttribute{Required: true, MarkdownDescription: "Plain Python or shell filename, unique within this event.", Validators: []validator.String{stringvalidator.LengthBetween(1, 255), stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(py|sh)$`), "Use a plain .py or .sh filename.")}},
		"content":      schema.StringAttribute{Required: true, MarkdownDescription: "Hook file content; applying configuration does not execute it.", Validators: []validator.String{stringvalidator.LengthBetween(1, 65536)}},
		"requirements": schema.ListAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Python dependencies; an empty list removes dependencies.", Validators: []validator.List{listvalidator.SizeAtMost(20), listvalidator.ValueStringsAre(stringvalidator.LengthBetween(1, 200), stringvalidator.RegexMatches(regexp.MustCompile(`^[^\s\r\n\x00](?:[^\r\n\x00]*[^\s\r\n\x00])?$`), "Use a trimmed single-line dependency."))}},
		"enabled":      schema.BoolAttribute{Required: true, MarkdownDescription: "Whether the hook is enabled for future agent events."},
	}}}
}
func (r *AgentResource) getHooks(ctx context.Context, id uuid.UUID, diags *diag.Diagnostics) []agentHookWire {
	response, err := r.client.GetHooksWithResponse(ctx, &client.GetHooksParams{AgentId: id})
	if err != nil {
		agentRelationshipError(diags, "Read agent hooks", 0, err)
		return nil
	}
	if response.JSON200 == nil {
		agentRelationshipError(diags, "Read agent hooks", response.StatusCode(), nil)
		return nil
	}
	var hooks []agentHookWire
	if err := json.Unmarshal(response.Body, &hooks); err != nil {
		diags.AddError("Invalid agent hooks", err.Error())
		return nil
	}
	seen := map[[2]string]bool{}
	for _, hook := range hooks {
		if hook.AgentID != id.String() {
			diags.AddError("Unexpected hook scope", "The API returned a hook outside the requested agent.")
			return nil
		}
		key := [2]string{hook.Event, hook.FileName}
		if seen[key] {
			diags.AddError("Ambiguous agent hooks", "The API returned more than one hook for an event and filename.")
			return nil
		}
		seen[key] = true
	}
	return hooks
}
func (r *AgentResource) readHooks(ctx context.Context, data *AgentResourceModel, diags *diag.Diagnostics) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid agent ID", err.Error())
		return
	}
	hooks := r.getHooks(ctx, id, diags)
	if diags.HasError() {
		return
	}
	values := make([]agentInlineHook, 0, len(hooks))
	for _, hook := range hooks {
		reqs := hook.Requirements
		if reqs == nil {
			reqs = []string{}
		}
		requirements, d := types.ListValueFrom(ctx, types.StringType, reqs)
		diags.Append(d...)
		values = append(values, agentInlineHook{Event: types.StringValue(hook.Event), FileName: types.StringValue(hook.FileName), Content: types.StringValue(hook.Content), Requirements: requirements, Enabled: types.BoolValue(hook.Enabled)})
	}
	value, d := types.SetValueFrom(ctx, agentHookType, values)
	diags.Append(d...)
	data.Hooks = value
}
func (r *AgentResource) writeHooks(ctx context.Context, plan, prior *AgentResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	if !agentConfigured(ctx, config, "hooks", diags) || plan.Hooks.Equal(prior.Hooks) || diags.HasError() {
		return
	}
	id, err := uuid.Parse(plan.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid agent ID", err.Error())
		return
	}
	var desired []agentInlineHook
	diags.Append(plan.Hooks.ElementsAs(ctx, &desired, false)...)
	if diags.HasError() {
		return
	}
	byKey := map[[2]string]agentInlineHook{}
	for _, hook := range desired {
		key := [2]string{hook.Event.ValueString(), hook.FileName.ValueString()}
		if _, exists := byKey[key]; exists {
			diags.AddError("Duplicate agent hook", "Use one hook per event and filename.")
			return
		}
		byKey[key] = hook
	}
	existing := r.getHooks(ctx, id, diags)
	if diags.HasError() {
		return
	}
	observed := map[[2]string]agentHookWire{}
	for _, hook := range existing {
		observed[[2]string{hook.Event, hook.FileName}] = hook
	}
	for _, hook := range desired {
		key := [2]string{hook.Event.ValueString(), hook.FileName.ValueString()}
		before, found := observed[key]
		var requirements []string
		diags.Append(hook.Requirements.ElementsAs(ctx, &requirements, false)...)
		if diags.HasError() {
			return
		}
		if requirements == nil {
			requirements = []string{}
		}
		patch := map[string]any{}
		if !found || before.Content != hook.Content.ValueString() {
			patch["content"] = hook.Content.ValueString()
		}
		if !found || before.Enabled != hook.Enabled.ValueBool() {
			patch["enabled"] = hook.Enabled.ValueBool()
		}
		if !found || !slices.Equal(before.Requirements, requirements) {
			patch["requirements"] = requirements
		}
		if !found {
			patch["agentId"] = id.String()
			patch["event"] = key[0]
			patch["fileName"] = key[1]
		}
		if len(patch) == 0 {
			continue
		}
		body, err := json.Marshal(patch)
		if err != nil {
			diags.AddError("Hook encoding failed", err.Error())
			return
		}
		if found {
			hookID, err := uuid.Parse(before.ID)
			if err != nil {
				diags.AddError("Invalid hook identity", err.Error())
				return
			}
			result, err := r.client.UpdateHookWithBodyWithResponse(ctx, hookID, "application/json", bytes.NewReader(body))
			if err != nil {
				agentRelationshipError(diags, "Update agent hook", 0, err)
				return
			}
			if result.JSON200 == nil {
				agentRelationshipError(diags, "Update agent hook", result.StatusCode(), nil)
				return
			}
		} else {
			result, err := r.client.CreateHookWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
			if err != nil {
				agentRelationshipError(diags, "Create agent hook", 0, err)
				return
			}
			if result.JSON200 == nil {
				agentRelationshipError(diags, "Create agent hook", result.StatusCode(), nil)
				return
			}
		}
	}
	for _, hook := range existing {
		if _, keep := byKey[[2]string{hook.Event, hook.FileName}]; keep {
			continue
		}
		hookID, err := uuid.Parse(hook.ID)
		if err != nil {
			diags.AddError("Invalid hook identity", err.Error())
			return
		}
		result, err := r.client.DeleteHookWithResponse(ctx, hookID)
		if err != nil {
			agentRelationshipError(diags, "Remove agent hook", 0, err)
			return
		}
		if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
			agentRelationshipError(diags, "Remove agent hook", result.StatusCode(), nil)
			return
		}
	}
}
func validateInlineHooks(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	var hooks types.Set
	diags.Append(config.GetAttribute(ctx, path.Root("hooks"), &hooks)...)
	if hooks.IsNull() || hooks.IsUnknown() || diags.HasError() {
		return
	}
	var values []agentInlineHook
	diags.Append(hooks.ElementsAs(ctx, &values, true)...)
	seen := map[[2]string]bool{}
	for _, hook := range values {
		if hook.Event.IsUnknown() || hook.FileName.IsUnknown() {
			continue
		}
		key := [2]string{hook.Event.ValueString(), hook.FileName.ValueString()}
		if seen[key] {
			diags.AddError("Duplicate agent hook", fmt.Sprintf("Use only one hook for event %q and file %q.", key[0], key[1]))
		}
		seen[key] = true
	}
}
