package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"time"
)

var _ resource.ResourceWithImportState = &DefaultUserLimitResource{}

func NewDefaultUserLimitResource() resource.Resource { return &DefaultUserLimitResource{} }

type DefaultUserLimitResource struct{ client *client.ClientWithResponses }
type DefaultUserLimitResourceModel struct {
	ID              types.String `tfsdk:"id"`
	EnvironmentID   types.String `tfsdk:"environment_id"`
	LimitValue      types.Int64  `tfsdk:"limit_value"`
	Model           types.List   `tfsdk:"model"`
	CleanupInterval types.String `tfsdk:"cleanup_interval"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

var default_user_limitAttrSpec = []AttrSpec{
	{TFName: "environment_id", JSONName: "environmentId", Kind: Scalar},
	{TFName: "limit_value", JSONName: "limitValue", Kind: Scalar},
	{TFName: "model", JSONName: "model", Kind: List},
	{TFName: "cleanup_interval", JSONName: "cleanupInterval", Kind: Scalar, OmitOnNull: true},
}

func (r *DefaultUserLimitResource) AttrSpecs() []AttrSpec { return default_user_limitAttrSpec }
func (r *DefaultUserLimitResource) APIShape() any         { return client.ListDefaultUserLimitsResponse{} }

// Permission projections, creator details and scheduler bookkeeping are owned by their respective APIs.
func (r *DefaultUserLimitResource) KnownIntentionallySkipped() []string {
	return []string{"lastCleanup"}
}
func (r *DefaultUserLimitResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_default_user_limit"
}
func (r *DefaultUserLimitResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Default token-cost limit for organization members, optionally overridden for one environment. Destroy removes this default and restores inheritance.", Attributes: map[string]schema.Attribute{
		"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Id."},
		"environment_id":   schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Environment id."},
		"limit_value":      schema.Int64Attribute{Required: true, MarkdownDescription: "Limit value."},
		"model":            schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Model."},
		"cleanup_interval": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Cleanup interval."},
		"organization_id":  schema.StringAttribute{Computed: true, MarkdownDescription: "Organization id."},
		"created_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "Created at."},
		"updated_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "Updated at."},
	}}
}
func (r *DefaultUserLimitResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	r.client = c
}
func (r *DefaultUserLimitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DefaultUserLimitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), default_user_limitAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_default_user_limit Create", patch, default_user_limitAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateDefaultUserLimitWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), result.Body))
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *DefaultUserLimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DefaultUserLimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	result, err := r.client.ListDefaultUserLimitsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) {
		resp.State.RemoveResource(ctx)
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	for _, item := range *result.JSON200 {
		if item.Id.String() == data.ID.ValueString() {
			body, err := json.Marshal(item)
			if err != nil {
				resp.Diagnostics.AddError("Response Encoding Failed", err.Error())
				return
			}
			r.flatten(ctx, &data, body, &resp.Diagnostics)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}
func (r *DefaultUserLimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DefaultUserLimitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, default_user_limitAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_default_user_limit Update", patch, default_user_limitAttrSpec)
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UpdateDefaultUserLimitWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Update Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), result.Body))
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *DefaultUserLimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DefaultUserLimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteDefaultUserLimitWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *DefaultUserLimitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *DefaultUserLimitResource) flatten(ctx context.Context, data *DefaultUserLimitResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID              *string    `json:"id"`
		EnvironmentID   *string    `json:"environmentId"`
		LimitValue      *int64     `json:"limitValue"`
		Model           []string   `json:"model"`
		CleanupInterval *string    `json:"cleanupInterval"`
		OrganizationID  *string    `json:"organizationId"`
		CreatedAt       *time.Time `json:"createdAt"`
		UpdatedAt       *time.Time `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	data.ID = types.StringPointerValue(value.ID)
	data.EnvironmentID = types.StringPointerValue(value.EnvironmentID)
	data.LimitValue = types.Int64PointerValue(value.LimitValue)
	var d diag.Diagnostics
	data.Model, d = types.ListValueFrom(ctx, types.StringType, value.Model)
	diags.Append(d...)
	data.CleanupInterval = types.StringPointerValue(value.CleanupInterval)
	data.OrganizationID = types.StringPointerValue(value.OrganizationID)
	data.CreatedAt = canonicalAPITimestamp(value.CreatedAt)
	data.UpdatedAt = canonicalAPITimestamp(value.UpdatedAt)
}
