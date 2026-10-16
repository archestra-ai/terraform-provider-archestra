package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
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

var _ resource.ResourceWithImportState = &EnvironmentResource{}

func NewEnvironmentResource() resource.Resource { return &EnvironmentResource{} }

type EnvironmentResource struct{ client *client.ClientWithResponses }
type EnvironmentResourceModel struct {
	ID                     types.String      `tfsdk:"id"`
	Name                   types.String      `tfsdk:"name"`
	Description            types.String      `tfsdk:"description"`
	Namespace              types.String      `tfsdk:"namespace"`
	ValidationRegex        types.String      `tfsdk:"validation_regex"`
	TrustedImageRegistries types.List        `tfsdk:"trusted_image_registries"`
	NetworkPolicy          types.Object      `tfsdk:"network_policy"`
	Labels                 []AgentLabelModel `tfsdk:"labels"`
	OrganizationID         types.String      `tfsdk:"organization_id"`
	SortOrder              types.Int64       `tfsdk:"sort_order"`
	CreatedAt              types.String      `tfsdk:"created_at"`
	UpdatedAt              types.String      `tfsdk:"updated_at"`
}

var environmentNetworkType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"egress_mode": types.StringType, "domain_preset": types.StringType, "allowed_domains": types.ListType{ElemType: types.StringType}, "allowed_cidrs": types.ListType{ElemType: types.StringType},
}}
var environmentAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "namespace", JSONName: "namespace", Kind: Scalar},
	{TFName: "validation_regex", JSONName: "validationRegex", Kind: Scalar},
	{TFName: "trusted_image_registries", JSONName: "trustedImageRegistries", Kind: List},
	{TFName: "labels", JSONName: "labels", Kind: Set, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
	{TFName: "network_policy", JSONName: "networkPolicy", Kind: AtomicObject, Children: []AttrSpec{
		{TFName: "egress_mode", JSONName: "egressMode", Kind: Scalar}, {TFName: "domain_preset", JSONName: "domainPreset", Kind: Scalar}, {TFName: "allowed_domains", JSONName: "allowedDomains", Kind: List}, {TFName: "allowed_cidrs", JSONName: "allowedCidrs", Kind: List},
	}},
}

func (r *EnvironmentResource) AttrSpecs() []AttrSpec               { return environmentAttrSpec }
func (r *EnvironmentResource) APIShape() any                       { return client.GetEnvironmentResponse{} }
func (r *EnvironmentResource) KnownIntentionallySkipped() []string { return nil }
func (r *EnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}
func (r *EnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Deployment environment and its network and container registry policies. Destroy deletes the environment; the API rejects deletion while resources remain assigned. Permissions are managed separately with archestra_resource_permission_policy. Requires an Archestra release containing GET /api/environments/{id}.", Attributes: map[string]schema.Attribute{
		"id":                       schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":                     schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 50)}},
		"description":              schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthAtMost(500)}},
		"namespace":                schema.StringAttribute{Optional: true, MarkdownDescription: "Existing Kubernetes namespace; changing it can redeploy assigned servers."},
		"validation_regex":         schema.StringAttribute{Optional: true, MarkdownDescription: "JavaScript regular expression for validating MCP configuration values."},
		"trusted_image_registries": schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Trusted container registry prefixes. Null or an empty list disables the registry restriction."},
		"labels":                   schema.SetNestedAttribute{Optional: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"key": schema.StringAttribute{Required: true}, "value": schema.StringAttribute{Required: true}}}},
		"network_policy": schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Explicit egress policy. Removing it restores inherited policy. Runtime capabilities determine enforcement.", Attributes: map[string]schema.Attribute{
			"egress_mode":     schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("off", "restricted", "unrestricted")}},
			"domain_preset":   schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("none", "common_dependencies", "package_managers")}},
			"allowed_domains": schema.ListAttribute{Required: true, ElementType: types.StringType},
			"allowed_cidrs":   schema.ListAttribute{Required: true, ElementType: types.StringType},
		}},
		"organization_id": schema.StringAttribute{Computed: true}, "sort_order": schema.Int64Attribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
	}}
}
func (r *EnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *EnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), environmentAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	created, err := r.client.CreateEnvironmentWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if created.JSON200 == nil {
		resp.Diagnostics.AddError("Environment Creation Failed", fmt.Sprintf("API returned status %d.", created.StatusCode()))
		return
	}
	data.ID = types.StringValue(created.JSON200.Id.String())
	// Persist the remote ID before any follow-up request can fail, so Terraform
	// can recover and destroy the created object after a partial apply.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *EnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *EnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, environmentAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Environment ID", err.Error())
		return
	}
	updated, err := r.client.UpdateEnvironmentWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if updated.StatusCode() != 200 {
		resp.Diagnostics.AddError("Environment Update Failed", fmt.Sprintf("API returned status %d. Refresh before retrying.", updated.StatusCode()))
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *EnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Environment ID", err.Error())
		return
	}
	deleted, err := r.client.DeleteEnvironmentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if deleted.StatusCode() != 200 && deleted.StatusCode() != 404 {
		resp.Diagnostics.AddError("Environment Deletion Failed", fmt.Sprintf("API returned status %d. Remove resource assignments before deleting an environment.", deleted.StatusCode()))
	}
}
func (r *EnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := uuid.Parse(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid Environment ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *EnvironmentResource) refresh(ctx context.Context, data *EnvironmentResourceModel, ds *diag.Diagnostics) bool {
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		return false
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		ds.AddError("Invalid Environment ID", err.Error())
		return false
	}
	response, err := r.client.GetEnvironmentWithResponse(ctx, id)
	if err != nil {
		ds.AddError("API Error", err.Error())
		return false
	}
	if response.StatusCode() == 404 {
		return false
	}
	if response.JSON200 == nil {
		ds.AddError("Environment Read Failed", fmt.Sprintf("API returned status %d.", response.StatusCode()))
		return false
	}
	wire := response.JSON200
	data.Name = types.StringValue(wire.Name)
	data.Description = types.StringPointerValue(wire.Description)
	data.Namespace = types.StringPointerValue(wire.Namespace)
	data.ValidationRegex = types.StringPointerValue(wire.ValidationRegex)
	data.OrganizationID = types.StringValue(wire.OrganizationId)
	data.SortOrder = types.Int64Value(int64(wire.SortOrder))
	data.CreatedAt = types.StringValue(wire.CreatedAt.Format(time.RFC3339Nano))
	data.UpdatedAt = types.StringValue(wire.UpdatedAt.Format(time.RFC3339Nano))
	var d diag.Diagnostics
	data.TrustedImageRegistries, d = types.ListValueFrom(ctx, types.StringType, wire.TrustedImageRegistries)
	ds.Append(d...)
	data.Labels = flattenAgentLabels(data.Labels, wire.Labels)
	data.NetworkPolicy = types.ObjectNull(environmentNetworkType.AttrTypes)
	if policy := wire.NetworkPolicy; policy != nil {
		domains, d := types.ListValueFrom(ctx, types.StringType, policy.AllowedDomains)
		ds.Append(d...)
		cidrs, d := types.ListValueFrom(ctx, types.StringType, policy.AllowedCidrs)
		ds.Append(d...)
		data.NetworkPolicy, d = types.ObjectValue(environmentNetworkType.AttrTypes, map[string]attr.Value{"egress_mode": types.StringValue(string(policy.EgressMode)), "domain_preset": types.StringValue(string(policy.DomainPreset)), "allowed_domains": domains, "allowed_cidrs": cidrs})
		ds.Append(d...)
	}
	return !ds.HasError()
}
