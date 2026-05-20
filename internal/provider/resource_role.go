package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}
var _ resource.ResourceWithValidateConfig = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

type RoleResource struct {
	client *client.ClientWithResponses
}

type RoleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Role        types.String `tfsdk:"role"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Permission  types.Map    `tfsdk:"permission"`
	Predefined  types.Bool   `tfsdk:"predefined"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *RoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Custom RBAC role granting a set of actions on selected resources. Enterprise Edition only; predefined roles are read-only and cannot be managed via Terraform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role identifier (base62 for custom roles).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"role": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Immutable role slug derived from `name` at create time.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable role name.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 50),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Free-form description, up to 200 characters.",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(200),
				},
			},
			"permission": schema.MapAttribute{
				Required:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
				MarkdownDescription: "Permission grants keyed by resource name; each value is a list of action verbs. Allowed actions: `admin`, `cancel`, `create`, `delete`, `enable`, `query`, `read`, `team-admin`, `update`.",
			},
			"predefined": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True for backend-managed roles that can't be modified through this resource.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last-update timestamp (RFC 3339), or null if never updated.",
			},
		},
	}
}

func (r *RoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig validates the permission map's action verbs at plan
// time. Backend rejects unknown actions with a 400; catching them in
// plan saves the round-trip.
func (r *RoleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Permission.IsNull() || data.Permission.IsUnknown() {
		return
	}

	allowed := map[string]struct{}{}
	for _, a := range validRoleActions {
		allowed[a] = struct{}{}
	}

	for resourceName, listVal := range data.Permission.Elements() {
		list, ok := listVal.(types.List)
		if !ok || list.IsNull() || list.IsUnknown() {
			continue
		}
		for i, elem := range list.Elements() {
			s, ok := elem.(types.String)
			if !ok || s.IsNull() || s.IsUnknown() {
				continue
			}
			if _, valid := allowed[s.ValueString()]; !valid {
				resp.Diagnostics.AddAttributeError(
					path.Root("permission").AtMapKey(resourceName).AtListIndex(i),
					"Invalid permission action",
					fmt.Sprintf("Action %q is not one of: %s", s.ValueString(), strings.Join(validRoleActions, ", ")),
				)
			}
		}
	}
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, roleAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_role Create", patch, roleAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateRoleWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if !r.flattenRoleBody(ctx, &data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if apiResp.JSON200.Predefined {
		resp.Diagnostics.AddError(
			"Predefined Role Not Manageable",
			fmt.Sprintf("Role %q is a backend-managed predefined role and cannot be managed by Terraform. "+
				"If you imported it by mistake, remove it from state with `terraform state rm`.",
				apiResp.JSON200.Role),
		)
		return
	}

	if !r.flattenRoleBody(ctx, &data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, roleAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		// Refresh from API: updated_at is Computed without
		// UseStateForUnknown, so plan carries Unknown and setting plan
		// into state would crash the framework.
		apiResp, gerr := r.client.GetRoleWithResponse(ctx, data.ID.ValueString())
		if gerr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to refresh role: %s", gerr))
			return
		}
		if IsNotFound(apiResp) {
			resp.Diagnostics.AddError(
				"Resource Deleted Outside Terraform",
				"The role disappeared between refresh and apply. Re-run `terraform apply` — the next refresh drops it from state and the plan recreates it.",
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
		if !r.flattenRoleBody(ctx, &data, apiResp.Body, &resp.Diagnostics) {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	LogPatch(ctx, "archestra_role Update", patch, roleAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.UpdateRoleWithBodyWithResponse(ctx, data.ID.ValueString(), "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update role: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Resource Deleted Outside Terraform",
			"The role was deleted on the backend between refresh and apply. Re-run `terraform apply` — the next refresh drops it from state and the plan recreates it.",
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

	if !r.flattenRoleBody(ctx, &data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role: %s", err))
		return
	}
	if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// roleAPIResponse mirrors the JSON shape of a role record. The
// generated client uses anonymous structs per endpoint and each
// permission map has a different enum type (Get/Create/Update each
// emit `*Role200Permission`), so structural-identity tricks don't
// compose across the three. JSON-roundtripping the raw body bytes
// into a single typed mirror is the canonical pattern in this
// codebase (see agent_shared.go::parseAgentResponse).
type roleAPIResponse struct {
	CreatedAt      time.Time           `json:"createdAt"`
	Description    *string             `json:"description"`
	Id             string              `json:"id"`
	Name           string              `json:"name"`
	OrganizationId string              `json:"organizationId"`
	Permission     map[string][]string `json:"permission"`
	Predefined     bool                `json:"predefined"`
	Role           string              `json:"role"`
	UpdatedAt      *time.Time          `json:"updatedAt"`
}

// flattenRoleBody decodes the raw API response body into state.
// Returns false on decode/projection error after appending diagnostics
// so the caller short-circuits.
func (r *RoleResource) flattenRoleBody(ctx context.Context, data *RoleResourceModel, body []byte, diags *diag.Diagnostics) bool {
	var src roleAPIResponse
	if err := json.Unmarshal(body, &src); err != nil {
		diags.AddError("API Response Decode Error", "Unable to decode role response: "+err.Error())
		return false
	}

	data.ID = types.StringValue(src.Id)
	data.Role = types.StringValue(src.Role)
	data.Name = types.StringValue(src.Name)
	optionalStringFromAPI(&data.Description, src.Description)
	data.Predefined = types.BoolValue(src.Predefined)
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	if src.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(src.UpdatedAt.Format(time.RFC3339))
	} else if !data.UpdatedAt.IsNull() {
		data.UpdatedAt = types.StringNull()
	}

	perm, pdiags := permissionMapToValue(ctx, src.Permission)
	diags.Append(pdiags...)
	if pdiags.HasError() {
		return false
	}
	data.Permission = perm
	return true
}

// permissionMapToValue projects the wire permission map into a
// types.Map(types.List(types.String)). Keys are sorted for stable
// debug logs; the framework compares Map values by key set, so order
// doesn't affect plan equality.
func permissionMapToValue(ctx context.Context, perms map[string][]string) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	listType := types.ListType{ElemType: types.StringType}

	if perms == nil {
		return types.MapNull(listType), diags
	}

	keys := make([]string, 0, len(perms))
	for k := range perms {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make(map[string]attr.Value, len(perms))
	for _, k := range keys {
		list, d := types.ListValueFrom(ctx, types.StringType, perms[k])
		diags.Append(d...)
		out[k] = list
	}
	m, d := types.MapValue(listType, out)
	diags.Append(d...)
	return m, diags
}
