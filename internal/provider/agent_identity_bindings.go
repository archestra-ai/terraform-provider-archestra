package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// identityBindingsMinimumAPIVersion is the first release with
// /api/agents/{id}/identity-bindings.
const identityBindingsMinimumAPIVersion = "1.5.0-rc.35"

type agentIdentityBinding struct {
	IdentityProviderID types.String `tfsdk:"identity_provider_id"`
	ExternalID         types.String `tfsdk:"external_id"`
}

var agentIdentityBindingType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"identity_provider_id": types.StringType,
	"external_id":          types.StringType,
}}

func agentIdentityBindingsAttribute() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional:            true,
		Computed:            true,
		PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
		Validators:          []validator.Set{setvalidator.SizeAtMost(50)},
		MarkdownDescription: "Identities this agent acts as at identity providers, such as its Microsoft Entra agent identity or Okta AI agent; omit to preserve or set an empty set to clear.",
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"identity_provider_id": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Identity provider the identity belongs to."},
			"external_id":          schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 255)}, MarkdownDescription: "The agent's ID at that identity provider, such as an Entra agent identity's application (client) ID."},
		}},
	}
}

// readIdentityBindings sets data.IdentityBindings from the API. An older
// platform has no identity-bindings route; the attribute is then null.
func (r *AgentResource) readIdentityBindings(ctx context.Context, id uuid.UUID, data *AgentResourceModel, diags *diag.Diagnostics) {
	// The raw response: a platform without the route answers a 404 whose
	// body the generated response type cannot parse.
	response, err := r.client.GetAgentIdentityBindings(ctx, id)
	if err != nil {
		agentRelationshipError(diags, "Read identity bindings", 0, err)
		return
	}
	status, body, err := readIdentityBindingsResponse(response)
	if err != nil {
		agentRelationshipError(diags, "Read identity bindings", 0, err)
		return
	}
	if routeMissing(status, body) {
		data.IdentityBindings = types.SetNull(agentIdentityBindingType)
		return
	}
	var result identityBindingsBody
	if status != http.StatusOK || json.Unmarshal(body, &result) != nil {
		diags.AddError("Agent relationship API error", fmt.Sprintf("Read identity bindings returned status %d: %s", status, body))
		return
	}
	bindings := make([]agentIdentityBinding, 0, len(result.Bindings))
	for _, binding := range result.Bindings {
		bindings = append(bindings, agentIdentityBinding{
			IdentityProviderID: types.StringValue(binding.IdentityProviderID),
			ExternalID:         types.StringValue(binding.ExternalID),
		})
	}
	set, d := types.SetValueFrom(ctx, agentIdentityBindingType, bindings)
	diags.Append(d...)
	data.IdentityBindings = set
}

// writeIdentityBindings replaces the agent's identity bindings with plan's.
func (r *AgentResource) writeIdentityBindings(ctx context.Context, id uuid.UUID, plan *AgentResourceModel, diags *diag.Diagnostics) bool {
	var bindings []agentIdentityBinding
	diags.Append(plan.IdentityBindings.ElementsAs(ctx, &bindings, false)...)
	if diags.HasError() {
		return false
	}
	request := identityBindingsBody{Bindings: make([]identityBindingJSON, 0, len(bindings))}
	for _, binding := range bindings {
		request.Bindings = append(request.Bindings, identityBindingJSON{
			IdentityProviderID: binding.IdentityProviderID.ValueString(),
			ExternalID:         binding.ExternalID.ValueString(),
		})
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		diags.AddError("Marshal Error", err.Error())
		return false
	}
	response, err := r.client.ReplaceAgentIdentityBindingsWithBody(ctx, id, "application/json", bytes.NewReader(encoded))
	if err != nil {
		agentRelationshipError(diags, "Replace identity bindings", 0, err)
		return false
	}
	status, body, err := readIdentityBindingsResponse(response)
	if err != nil {
		agentRelationshipError(diags, "Replace identity bindings", 0, err)
		return false
	}
	if routeMissing(status, body) {
		diags.AddError("Identity bindings unsupported", "identity_bindings requires Archestra "+identityBindingsMinimumAPIVersion+" or later; remove it for this platform.")
		return false
	}
	if status != http.StatusOK {
		diags.AddError("Agent relationship API error", fmt.Sprintf("Replace identity bindings returned status %d: %s", status, body))
		return false
	}
	return true
}

type identityBindingJSON struct {
	IdentityProviderID string `json:"identityProviderId"`
	ExternalID         string `json:"externalId"`
}

type identityBindingsBody struct {
	Bindings []identityBindingJSON `json:"bindings"`
}

func readIdentityBindingsResponse(response *http.Response) (int, []byte, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return response.StatusCode, body, err
}

// routeMissing reports a 404 from a platform without the route. The
// platform's own 404s (an unknown identity provider) carry
// {"error": {"message": ...}}.
func routeMissing(status int, body []byte) bool {
	if status != http.StatusNotFound {
		return false
	}
	var platform struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	return json.Unmarshal(body, &platform) != nil || platform.Error.Message == ""
}
