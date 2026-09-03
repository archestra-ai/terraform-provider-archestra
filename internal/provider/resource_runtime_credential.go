package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ resource.Resource                   = &RuntimeCredentialResource{}
	_ resource.ResourceWithImportState    = &RuntimeCredentialResource{}
	_ resource.ResourceWithValidateConfig = &RuntimeCredentialResource{}
)

func NewRuntimeCredentialResource() resource.Resource { return &RuntimeCredentialResource{} }

type RuntimeCredentialResource struct {
	client *client.ClientWithResponses
}

// RuntimeCredentialResourceModel models one runtime credential definition
// (a named secret slot that agent `runtime.credentials` bindings
// reference by key) plus, optionally, its organization-scoped value.
type RuntimeCredentialResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Key                    types.String `tfsdk:"key"`
	Name                   types.String `tfsdk:"name"`
	Description            types.String `tfsdk:"description"`
	Icon                   types.String `tfsdk:"icon"`
	AllowPersonal          types.Bool   `tfsdk:"allow_personal"`
	AllowOrganization      types.Bool   `tfsdk:"allow_organization"`
	OrganizationValue      types.String `tfsdk:"organization_value"`
	OrganizationConfigured types.Bool   `tfsdk:"organization_configured"`
}

func (r *RuntimeCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runtime_credential"
}

func (r *RuntimeCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Runtime credential definition for Agent Runtime runs. Declares a named secret slot that `runtime.credentials` bindings reference by `key`; the value is deposited per-user in the UI (`allow_personal`) or once for the whole organization via `organization_value` (`allow_organization`). Requires an Agent Runtime backend on the platform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `key` — the backend addresses definitions by key",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Stable credential identifier referenced by `runtime.credentials[].credential_id` (lowercase letter first; lowercase letters, numbers, dots, dashes, underscores)",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(runtimeCredentialKeyRegexp, "must start with a lowercase letter and use lowercase letters, numbers, dots, dashes, or underscores"),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable credential name",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "How to obtain the credential, shown when prompting users",
			},
			"icon": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Emoji or base64 image data URL",
			},
			"allow_personal": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Each user deposits their own value (default `true`). Exactly one of `allow_personal` / `allow_organization` must be `true`.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"allow_organization": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "One organization-level value serves every user (default `false`). Exactly one of `allow_personal` / `allow_organization` must be `true`.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"organization_value": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "The organization-scoped secret value. Only valid with `allow_organization = true`. Write-only: the backend never echoes it, so out-of-band rotation is invisible to Terraform (out-of-band *deletion* is detected via `organization_configured`). Removing the attribute disconnects the stored value.",
			},
			"organization_configured": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether an organization-scoped value is currently deposited",
			},
		},
	}
}

func (r *RuntimeCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

// ValidateConfig enforces the backend's exactly-one-scope check constraint at
// plan time, and that `organization_value` only accompanies the organization
// scope — both otherwise surface as apply-time 400s.
func (r *RuntimeCredentialResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data RuntimeCredentialResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.AllowPersonal.IsUnknown() || data.AllowOrganization.IsUnknown() {
		return
	}

	// Nulls take the schema defaults (personal on, organization off).
	allowPersonal := data.AllowPersonal.IsNull() || data.AllowPersonal.ValueBool()
	allowOrganization := !data.AllowOrganization.IsNull() && data.AllowOrganization.ValueBool()

	if allowPersonal == allowOrganization {
		resp.Diagnostics.AddAttributeError(
			path.Root("allow_personal"),
			"Invalid Attribute Combination",
			"Exactly one of allow_personal and allow_organization must be true "+
				"(set allow_personal = false when enabling allow_organization).",
		)
	}
	if !data.OrganizationValue.IsNull() && !data.OrganizationValue.IsUnknown() && !allowOrganization {
		resp.Diagnostics.AddAttributeError(
			path.Root("organization_value"),
			"Invalid Attribute Combination",
			`organization_value requires allow_organization = true`,
		)
	}
}

func (r *RuntimeCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RuntimeCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, runtimeCredentialAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_runtime_credential Create", patch, runtimeCredentialAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateRuntimeCredentialWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create runtime credential: %s", err))
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError(
			"Agent Runtime Not Enabled",
			"The backend returned 404 for the runtime-credential API. Configure an Agent Runtime backend before managing runtime credentials.",
		)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	plan.ID = types.StringValue(apiResp.JSON200.Key)
	plan.Description = types.StringValue(apiResp.JSON200.Description)

	if !plan.OrganizationValue.IsNull() {
		if !r.putOrganizationValue(ctx, plan.Key.ValueString(), plan.OrganizationValue.ValueString(), &resp.Diagnostics) {
			// Keep the definition in state so a retry converges instead of
			// orphaning the created row.
			plan.OrganizationConfigured = types.BoolValue(false)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
		plan.OrganizationConfigured = types.BoolValue(true)
	} else {
		plan.OrganizationConfigured = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RuntimeCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RuntimeCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	view, ok := r.findByKey(ctx, data.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Key = types.StringValue(view.Key)
	data.Name = types.StringValue(view.Name)
	data.Description = types.StringValue(view.Description)
	optionalStringFromAPI(&data.Icon, view.Icon)
	data.AllowPersonal = types.BoolValue(view.AllowPersonal)
	data.AllowOrganization = types.BoolValue(view.AllowOrganization)
	data.OrganizationConfigured = types.BoolValue(view.OrganizationConfigured)

	// The value itself is write-only (never echoed), so preserve it from
	// prior state — except when the backend no longer holds one, which must
	// surface as drift.
	if !view.OrganizationConfigured {
		data.OrganizationValue = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuntimeCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state RuntimeCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := state.Key.ValueString()

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, runtimeCredentialAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// key/name/allow_* are RequiresReplace, so only the PATCH-able fields
	// (description, icon) can appear here.
	if len(patch) > 0 {
		LogPatch(ctx, "archestra_runtime_credential Update", patch, runtimeCredentialAttrSpec)
		body, err := json.Marshal(patch)
		if err != nil {
			resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("unable to marshal request body: %s", err))
			return
		}
		apiResp, err := r.client.UpdateRuntimeCredentialWithBodyWithResponse(ctx, key, "application/json", bytes.NewReader(body))
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update runtime credential: %s", err))
			return
		}
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}
		plan.Description = types.StringValue(apiResp.JSON200.Description)
	}

	switch {
	case !plan.OrganizationValue.IsNull() && !plan.OrganizationValue.Equal(state.OrganizationValue):
		if !r.putOrganizationValue(ctx, key, plan.OrganizationValue.ValueString(), &resp.Diagnostics) {
			return
		}
		plan.OrganizationConfigured = types.BoolValue(true)
	case plan.OrganizationValue.IsNull() && !state.OrganizationValue.IsNull():
		apiResp, err := r.client.DeleteOrganizationRuntimeCredentialConnectionWithResponse(ctx, key)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to disconnect organization credential value: %s", err))
			return
		}
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}
		plan.OrganizationConfigured = types.BoolValue(false)
	default:
		plan.OrganizationConfigured = state.OrganizationConfigured
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RuntimeCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RuntimeCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The backend cascades stored connection values on definition delete.
	apiResp, err := r.client.DeleteRuntimeCredentialWithResponse(ctx, data.Key.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete runtime credential: %s", err))
		return
	}
	if apiResp.StatusCode() == 409 {
		resp.Diagnostics.AddError(
			"Runtime Credential In Use",
			"An agent's runtime.credentials still references this credential. Remove those bindings first, then destroy.",
		)
		return
	}
	if apiResp.JSON200 == nil && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

// ImportState imports by credential key. `organization_value` cannot be
// recovered (write-only) — a configured value shows one apply that re-sends it.
func (r *RuntimeCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// AttrSpecs implements resourceWithAttrSpec (see specdrift_test.go).
func (r *RuntimeCredentialResource) AttrSpecs() []AttrSpec { return runtimeCredentialAttrSpec }

func (r *RuntimeCredentialResource) APIShape() any {
	return client.ListRuntimeCredentialsResponse{}
}

// KnownIntentionallySkipped — wire fields deliberately not modeled:
//   - builtIn: platform-seeded definitions only; a Terraform-created
//     definition is never built-in, so the flag carries no signal here.
//   - personalConfigured: whether the *calling API-key user* deposited a
//     personal value — caller-relative, meaningless as shared state.
func (r *RuntimeCredentialResource) KnownIntentionallySkipped() []string {
	return []string{"builtIn", "personalConfigured"}
}

// runtimeCredentialAttrSpec declares the wire shape. key/name/allow_* only
// ever appear in Create patches (RequiresReplace keeps them out of Update
// ones, matching the PATCH endpoint's description+icon-only contract).
var runtimeCredentialAttrSpec = []AttrSpec{
	{TFName: "key", JSONName: "key", Kind: Scalar},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar, OmitOnNull: true},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "allow_personal", JSONName: "allowPersonal", Kind: Scalar, OmitOnNull: true},
	{TFName: "allow_organization", JSONName: "allowOrganization", Kind: Scalar, OmitOnNull: true},
	// Deposited via PUT /api/runtime-credentials/:key/organization, never
	// part of the definition body.
	{TFName: "organization_value", Kind: Synthetic, Sensitive: true},
}

var runtimeCredentialKeyRegexp = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)

// runtimeCredentialView is a named copy of the list endpoint's element
// shape so findByKey can return it (struct conversion ignores field tags).
type runtimeCredentialView struct {
	AllowOrganization      bool
	AllowPersonal          bool
	BuiltIn                bool
	Description            string
	Icon                   *string
	Key                    string
	Name                   string
	OrganizationConfigured bool
	PersonalConfigured     bool
}

// findByKey resolves one definition from the list endpoint (the API has no
// GET-by-key). ok=false means not found (resource gone or runtime disabled).
func (r *RuntimeCredentialResource) findByKey(ctx context.Context, key string, diags *diag.Diagnostics) (runtimeCredentialView, bool) {
	apiResp, err := r.client.ListRuntimeCredentialsWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to list runtime credentials: %s", err))
		return runtimeCredentialView{}, false
	}
	if apiResp.StatusCode() == 404 {
		return runtimeCredentialView{}, false
	}
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return runtimeCredentialView{}, false
	}
	for _, view := range *apiResp.JSON200 {
		if view.Key == key {
			return runtimeCredentialView(view), true
		}
	}
	return runtimeCredentialView{}, false
}

func (r *RuntimeCredentialResource) putOrganizationValue(ctx context.Context, key, value string, diags *diag.Diagnostics) bool {
	apiResp, err := r.client.SetOrganizationRuntimeCredentialConnectionWithResponse(ctx, key,
		client.SetOrganizationRuntimeCredentialConnectionJSONRequestBody{Value: value})
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to set organization credential value: %s", err))
		return false
	}
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK setting organization credential value, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return false
	}
	return true
}
