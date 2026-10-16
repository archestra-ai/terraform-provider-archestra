package provider

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &ResourcePermissionPolicyResource{}
	_ resource.ResourceWithImportState    = &ResourcePermissionPolicyResource{}
	_ resource.ResourceWithValidateConfig = &ResourcePermissionPolicyResource{}
)

func NewResourcePermissionPolicyResource() resource.Resource {
	return &ResourcePermissionPolicyResource{}
}

type ResourcePermissionPolicyResource struct{ client *client.ClientWithResponses }
type ResourcePermissionPolicyResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ResourceType     types.String `tfsdk:"resource_type"`
	ResourceID       types.String `tfsdk:"resource_id"`
	Grants           types.Set    `tfsdk:"grants"`
	Revision         types.Int64  `tfsdk:"revision"`
	OwnerID          types.String `tfsdk:"owner_id"`
	EffectiveActions types.Set    `tfsdk:"effective_actions"`
}
type permissionGrantModel struct {
	SubjectType types.String `tfsdk:"subject_type"`
	SubjectID   types.String `tfsdk:"subject_id"`
	Actions     types.Set    `tfsdk:"actions"`
}
type permissionGrantWire struct {
	Subject struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"subject"`
	Actions []string `json:"actions"`
}
type permissionPolicyWire struct {
	Resource         string                `json:"resource"`
	Scope            string                `json:"scope"`
	Revision         int64                 `json:"revision"`
	OwnerID          *string               `json:"ownerId"`
	Grants           []permissionGrantWire `json:"grants"`
	EffectiveActions []string              `json:"effectiveActions"`
}

var permissionGrantObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"subject_type": types.StringType, "subject_id": types.StringType, "actions": types.SetType{ElemType: types.StringType},
}}
var permissionResourceTypes = []string{"agent", "mcpGateway", "mcpRegistry", "skill", "app", "llmModel", "project", "conversation", "agentRun", "plugin", "knowledgeBase", "knowledgeConnector", "knowledgeFile", "llmVirtualKey", "llmProviderApiKey", "externalAgent", "oauthClient", "environment", "serviceAccount", "credential"}
var permissionActions = []string{"read", "use", "update", "delete", "manage-permissions", "configure-deployment-spec"}

func (r *ResourcePermissionPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_permission_policy"
}
func (r *ResourcePermissionPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Authoritative direct grants for one object or all objects of one resource type. Declare one policy per target; inherited grants are unaffected. Destroy revokes every direct grant on the target. Updates use the last observed revision and reject concurrent changes.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, MarkdownDescription: "Composite resource_type/resource_id identifier."},
			"resource_type": schema.StringAttribute{Required: true, MarkdownDescription: "API resource type whose direct grants are managed.", Validators: []validator.String{stringvalidator.OneOf(permissionResourceTypes...)}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"resource_id":   schema.StringAttribute{Required: true, MarkdownDescription: "Object UUID, or * to manage the policy for all objects of this resource type.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"grants": schema.SetNestedAttribute{Required: true, MarkdownDescription: "Complete set of direct principal grants; an empty set revokes all direct grants.", Validators: []validator.Set{setvalidator.SizeAtMost(200)}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"subject_type": schema.StringAttribute{Required: true, MarkdownDescription: "Principal type.", Validators: []validator.String{stringvalidator.OneOf("user", "team", "serviceAccount", "role", "organization")}},
				"subject_id":   schema.StringAttribute{Required: true, MarkdownDescription: "Principal ID; use * for the organization principal.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
				"actions":      schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Actions granted to this principal.", Validators: []validator.Set{setvalidator.SizeBetween(1, 6), setvalidator.ValueStringsAre(stringvalidator.OneOf(permissionActions...))}},
			}}},
			"revision":          schema.Int64Attribute{Computed: true, MarkdownDescription: "Last observed policy revision used for optimistic concurrency."},
			"owner_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Object owner, when present."},
			"effective_actions": schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Current caller's effective actions, including inherited grants."},
		},
	}
}
func (r *ResourcePermissionPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got %T", req.ProviderData))
		return
	}
	r.client = c
}
func (r *ResourcePermissionPolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var grants types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("grants"), &grants)...)
	if grants.IsUnknown() || grants.IsNull() || resp.Diagnostics.HasError() {
		return
	}
	seen := map[string]bool{}
	for _, value := range grants.Elements() {
		object, ok := value.(types.Object)
		if !ok || object.IsUnknown() || object.IsNull() {
			continue
		}
		values := object.Attributes()
		kind := values["subject_type"].(types.String)
		id := values["subject_id"].(types.String)
		if kind.IsUnknown() || id.IsUnknown() {
			continue
		}
		key := kind.ValueString() + ":" + id.ValueString()
		if seen[key] {
			resp.Diagnostics.AddAttributeError(path.Root("grants"), "Duplicate Principal", "Combine a principal's actions in one grant.")
		}
		seen[key] = true
		if kind.ValueString() == "organization" && id.ValueString() != "*" {
			resp.Diagnostics.AddAttributeError(path.Root("grants"), "Invalid Organization Principal", "The organization subject_id must be *.")
		}
	}
}
func (r *ResourcePermissionPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ResourcePermissionPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, status := r.readPolicy(ctx, &plan, &resp.Diagnostics)
	if current == nil {
		if status == 404 {
			resp.Diagnostics.AddError("Target Not Found", "Create the target object before declaring its permission policy.")
		}
		return
	}
	grants := permissionGrantsFromPlan(ctx, plan.Grants, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	updated := r.writePolicy(ctx, &plan, current.Revision, grants, &resp.Diagnostics)
	if updated == nil {
		return
	}
	setPermissionPolicyState(ctx, &plan, updated, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *ResourcePermissionPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ResourcePermissionPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, status := r.readPolicy(ctx, &state, &resp.Diagnostics)
	if status == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if current == nil {
		return
	}
	setPermissionPolicyState(ctx, &state, current, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	}
}
func (r *ResourcePermissionPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior ResourcePermissionPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	grants := permissionGrantsFromPlan(ctx, plan.Grants, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	updated := r.writePolicy(ctx, &plan, prior.Revision.ValueInt64(), grants, &resp.Diagnostics)
	if updated == nil {
		return
	}
	setPermissionPolicyState(ctx, &plan, updated, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *ResourcePermissionPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ResourcePermissionPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, status := r.readPolicy(ctx, &state, &resp.Diagnostics)
	if status == 404 {
		return
	}
	if current == nil {
		return
	}
	if current.Revision != state.Revision.ValueInt64() {
		resp.Diagnostics.AddError("Policy Revision Conflict", "Direct grants changed after the last refresh. Refresh and review the policy before destroying it.")
		return
	}
	r.writePolicy(ctx, &state, state.Revision.ValueInt64(), []permissionGrantWire{}, &resp.Diagnostics)
}
func (r *ResourcePermissionPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import Identifier", "Expected resource_type/resource_id, for example environment/00000000-0000-0000-0000-000000000001.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_type"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_id"), parts[1])...)
}
func (r *ResourcePermissionPolicyResource) readPolicy(ctx context.Context, state *ResourcePermissionPolicyResourceModel, diags *diag.Diagnostics) (*permissionPolicyWire, int) {
	response, err := r.client.GetResourcePermissionsWithResponse(ctx, client.GetResourcePermissionsParamsResource(state.ResourceType.ValueString()), state.ResourceID.ValueString())
	if err != nil {
		diags.AddError("API Error", err.Error())
		return nil, 0
	}
	status := response.StatusCode()
	if status == 404 {
		return nil, status
	}
	if status != 200 {
		diags.AddError("Permission Policy Read Failed", fmt.Sprintf("API returned status %d.", status))
		return nil, status
	}
	var policy permissionPolicyWire
	if err := json.Unmarshal(response.Body, &policy); err != nil {
		diags.AddError("API Response Decode Error", err.Error())
		return nil, status
	}
	return &policy, status
}
func (r *ResourcePermissionPolicyResource) writePolicy(ctx context.Context, state *ResourcePermissionPolicyResourceModel, revision int64, grants []permissionGrantWire, diags *diag.Diagnostics) *permissionPolicyWire {
	body, err := json.Marshal(struct {
		Revision int64                 `json:"revision"`
		Grants   []permissionGrantWire `json:"grants"`
	}{revision, grants})
	if err != nil {
		diags.AddError("Policy Encoding Failed", err.Error())
		return nil
	}
	response, err := r.client.UpdateResourcePermissionsWithBodyWithResponse(ctx, client.UpdateResourcePermissionsParamsResource(state.ResourceType.ValueString()), state.ResourceID.ValueString(), "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("API Error", err.Error())
		return nil
	}
	if response.StatusCode() == 409 {
		diags.AddError("Policy Revision Conflict", "The policy changed after the last read. Refresh and review the new grants before applying again.")
		return nil
	}
	if response.StatusCode() != 200 {
		diags.AddError("Permission Policy Update Failed", fmt.Sprintf("API returned status %d: %s", response.StatusCode(), permissionPolicyErrorMessage(response.Body)))
		return nil
	}
	var policy permissionPolicyWire
	if err := json.Unmarshal(response.Body, &policy); err != nil {
		diags.AddError("API Response Decode Error", err.Error())
		return nil
	}
	return &policy
}
func permissionGrantsFromPlan(ctx context.Context, value types.Set, diags *diag.Diagnostics) []permissionGrantWire {
	var models []permissionGrantModel
	diags.Append(value.ElementsAs(ctx, &models, false)...)
	out := make([]permissionGrantWire, 0, len(models))
	for _, model := range models {
		grant := permissionGrantWire{}
		grant.Subject.Type = model.SubjectType.ValueString()
		grant.Subject.ID = model.SubjectID.ValueString()
		diags.Append(model.Actions.ElementsAs(ctx, &grant.Actions, false)...)
		out = append(out, grant)
	}
	return out
}
func setPermissionPolicyState(ctx context.Context, state *ResourcePermissionPolicyResourceModel, policy *permissionPolicyWire, diags *diag.Diagnostics) {
	state.ID = types.StringValue(state.ResourceType.ValueString() + "/" + state.ResourceID.ValueString())
	state.Revision = types.Int64Value(policy.Revision)
	state.OwnerID = types.StringPointerValue(policy.OwnerID)
	var actionsDiags diag.Diagnostics
	state.EffectiveActions, actionsDiags = types.SetValueFrom(ctx, types.StringType, policy.EffectiveActions)
	diags.Append(actionsDiags...)
	grants := make([]attr.Value, 0, len(policy.Grants))
	for _, grant := range policy.Grants {
		actions, d := types.SetValueFrom(ctx, types.StringType, grant.Actions)
		diags.Append(d...)
		value, d := types.ObjectValue(permissionGrantObjectType.AttrTypes, map[string]attr.Value{"subject_type": types.StringValue(grant.Subject.Type), "subject_id": types.StringValue(grant.Subject.ID), "actions": actions})
		diags.Append(d...)
		grants = append(grants, value)
	}
	state.Grants, actionsDiags = types.SetValue(permissionGrantObjectType, grants)
	diags.Append(actionsDiags...)
}
func (r *ResourcePermissionPolicyResource) APIShape() any {
	return client.GetResourcePermissionsResponse{}
}

// Resource/scope are represented by the resource_type/resource_id path fields.
// Display names, inherited grants and actor capabilities are computed by the
// authorization system; this resource owns direct grants only.
func (r *ResourcePermissionPolicyResource) KnownIntentionallySkipped() []string {
	return []string{"resource", "scope", "name", "inheritedGrants", "canShareOrganizationWide", "actorSubjects"}
}

func permissionPolicyErrorMessage(body []byte) string {
	var response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &response) != nil || response.Error.Message == "" {
		return "No successful update was recorded."
	}
	return response.Error.Message
}
