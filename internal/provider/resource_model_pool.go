package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ resource.ResourceWithImportState    = &ModelPoolResource{}
	_ resource.ResourceWithValidateConfig = &ModelPoolResource{}
)

func NewModelPoolResource() resource.Resource { return &ModelPoolResource{} }

type ModelPoolResource struct{ client *client.ModelPoolClient }
type ModelPoolResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Members          types.Set    `tfsdk:"members"`
	VirtualAPIKeyIDs types.Set    `tfsdk:"virtual_api_key_ids"`
}
type modelPoolMemberModel struct {
	ProviderAPIKeyID types.String `tfsdk:"provider_api_key_id"`
	ModelID          types.String `tfsdk:"model_id"`
	Priority         types.Int64  `tfsdk:"priority"`
	Weight           types.Int64  `tfsdk:"weight"`
	Enabled          types.Bool   `tfsdk:"enabled"`
}

var modelPoolMemberType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"provider_api_key_id": types.StringType, "model_id": types.StringType,
	"priority": types.Int64Type, "weight": types.Int64Type, "enabled": types.BoolType,
}}
var modelPoolAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "members", JSONName: "members", Kind: Set, Children: []AttrSpec{
		{TFName: "provider_api_key_id", JSONName: "providerApiKeyId", Kind: Scalar},
		{TFName: "model_id", JSONName: "modelId", Kind: Scalar},
		{TFName: "priority", JSONName: "priority", Kind: Scalar},
		{TFName: "weight", JSONName: "weight", Kind: Scalar},
		{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
	}},
	{TFName: "virtual_api_key_ids", JSONName: "virtualApiKeyIds", Kind: Set},
}

func (r *ModelPoolResource) AttrSpecs() []AttrSpec { return modelPoolAttrSpec }
func (r *ModelPoolResource) APIShape() any         { return client.GetModelPoolsResponse{} }
func (r *ModelPoolResource) KnownIntentionallySkipped() []string {
	// Organization ownership and audit timestamps are server bookkeeping.
	return []string{"organizationId", "createdAt", "updatedAt"}
}
func (r *ModelPoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_pool"
}
func (r *ModelPoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Model name balanced across shared provider keys with priority, weights and server-managed failover. Requires Archestra's model-pools API (PR #9681) and ARCHESTRA_LLM_MODEL_POOLS_ENABLED or ARCHESTRA_BETA. Members and explicit virtual-key grants are authoritative; import an existing pool by UUID before managing it.", Attributes: map[string]schema.Attribute{
		"id":   schema.StringAttribute{Computed: true, MarkdownDescription: "Model pool UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name": schema.StringAttribute{Required: true, MarkdownDescription: "Model name callers request.", Validators: modelPoolStringValidators()},
		"members": schema.SetNestedAttribute{Required: true, MarkdownDescription: "Shared provider keys and their upstream model identifiers.", Validators: []validator.Set{setvalidator.SizeBetween(1, 50)}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"provider_api_key_id": schema.StringAttribute{Required: true, MarkdownDescription: "Shared ModelProvider UUID in this organization.", Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical provider key UUID.")}},
			"model_id":            schema.StringAttribute{Required: true, MarkdownDescription: "Upstream model or deployment identifier for this provider key.", Validators: modelPoolStringValidators()},
			"priority":            schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), MarkdownDescription: "Order from 1 to 100; higher orders are tried after lower orders fail.", Validators: []validator.Int64{int64validator.Between(1, 100)}},
			"weight":              schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), MarkdownDescription: "Relative traffic weight from 1 to 1000 within the same priority.", Validators: []validator.Int64{int64validator.Between(1, 1000)}},
			"enabled":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether this member may receive traffic."},
		}}},
		"virtual_api_key_ids": schema.SetAttribute{Optional: true, Computed: true, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})), ElementType: types.StringType, MarkdownDescription: "Additional standard virtual key UUIDs explicitly allowed to call this pool.", Validators: []validator.Set{setvalidator.SizeAtMost(500), setvalidator.ValueStringsAre(stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use lowercase canonical virtual key UUIDs."))}},
	}}
}
func (r *ModelPoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	var err error
	r.client, err = client.NewModelPoolClient(api)
	if err != nil {
		resp.Diagnostics.AddError("Model Pool Client Configuration Failed", err.Error())
	}
}
func (r *ModelPoolResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ModelPoolResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Members.IsNull() || data.Members.IsUnknown() {
		return
	}
	var members []modelPoolMemberModel
	resp.Diagnostics.Append(data.Members.ElementsAs(ctx, &members, false)...)
	seen := map[string]bool{}
	for _, m := range members {
		if m.ProviderAPIKeyID.IsUnknown() || m.ModelID.IsUnknown() {
			continue
		}
		key := m.ProviderAPIKeyID.ValueString() + ":" + m.ModelID.ValueString()
		if seen[key] {
			resp.Diagnostics.AddAttributeError(path.Root("members"), "Duplicate Pool Member", "A provider key can serve a model in a pool only once.")
		}
		seen[key] = true
	}
}
func (r *ModelPoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelPoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := modelPoolBody(ctx, req.Plan.Raw, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	pool, err := r.client.Write(ctx, "", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("Model Pool Creation Failed", err.Error())
		return
	}
	if !canonicalUUIDRegexp.MatchString(pool.ID) {
		resp.Diagnostics.AddError("Invalid Model Pool Response", "Creation did not return a canonical pool UUID.")
		return
	}
	// Retain the created identity even if response mapping fails, so a later
	// refresh or destroy can recover the remote pool instead of orphaning it.
	data.ID = types.StringValue(pool.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.flatten(ctx, &data, pool, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *ModelPoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelPoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	pool := r.find(ctx, data.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if pool == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	r.flatten(ctx, &data, pool, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *ModelPoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ModelPoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// PUT replaces the entire pool; a diff-only merge patch would reset members
	// or grants. Use the shared encoder against a typed null to send the full plan.
	body := modelPoolBody(ctx, req.Plan.Raw, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	pool, err := r.client.Write(ctx, data.ID.ValueString(), bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("Model Pool Update Failed", err.Error())
		return
	}
	if pool.ID != data.ID.ValueString() {
		resp.Diagnostics.AddError("Invalid Model Pool Response", "Update returned a different pool UUID.")
		return
	}
	r.flatten(ctx, &data, pool, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *ModelPoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelPoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.find(ctx, data.ID.ValueString(), &resp.Diagnostics) == nil || resp.Diagnostics.HasError() {
		return
	}
	status, err := r.client.Delete(ctx, data.ID.ValueString())
	if status == 404 {
		// A disabled beta API also returns 404; only a successful list can prove
		// that the pool disappeared, including a concurrent deletion.
		if r.find(ctx, data.ID.ValueString(), &resp.Diagnostics) == nil && !resp.Diagnostics.HasError() {
			return
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Model Pool Deletion Failed", err.Error())
	}
}
func (r *ModelPoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !canonicalUUIDRegexp.MatchString(req.ID) {
		resp.Diagnostics.AddError("Invalid Model Pool Import ID", "Import using the pool's lowercase canonical UUID from the UI or GET /api/model-pools.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *ModelPoolResource) find(ctx context.Context, id string, diags *diag.Diagnostics) *client.ModelPool {
	pools, err := r.client.List(ctx)
	if err != nil {
		diags.AddError("Model Pool Read Failed", err.Error())
		return nil
	}
	for _, pool := range pools {
		if pool.ID == id {
			return &pool
		}
	}
	return nil
}
func (r *ModelPoolResource) flatten(ctx context.Context, data *ModelPoolResourceModel, pool *client.ModelPool, diags *diag.Diagnostics) {
	if !canonicalUUIDRegexp.MatchString(pool.ID) || pool.Name == "" || pool.Members == nil || pool.VirtualAPIKeyIDs == nil {
		diags.AddError("Invalid Model Pool Response", "Expected a pool UUID, name, members and virtual-key grants.")
		return
	}
	data.ID = types.StringValue(pool.ID)
	data.Name = types.StringValue(pool.Name)
	members := make([]modelPoolMemberModel, 0, len(pool.Members))
	for _, m := range pool.Members {
		members = append(members, modelPoolMemberModel{ProviderAPIKeyID: types.StringValue(m.ProviderAPIKeyID), ModelID: types.StringValue(m.ModelID), Priority: types.Int64Value(m.Priority), Weight: types.Int64Value(m.Weight), Enabled: types.BoolValue(m.Enabled)})
	}
	var d diag.Diagnostics
	data.Members, d = types.SetValueFrom(ctx, modelPoolMemberType, members)
	diags.Append(d...)
	data.VirtualAPIKeyIDs, d = types.SetValueFrom(ctx, types.StringType, pool.VirtualAPIKeyIDs)
	diags.Append(d...)
}
func modelPoolBody(ctx context.Context, plan tftypes.Value, diags *diag.Diagnostics) []byte {
	fields := MergePatch(ctx, plan, tftypes.NewValue(plan.Type(), nil), modelPoolAttrSpec, diags)
	if diags.HasError() {
		return nil
	}
	body, err := json.Marshal(fields)
	if err != nil {
		diags.AddError("Model Pool Encoding Failed", err.Error())
	}
	return body
}
func modelPoolStringValidators() []validator.String {
	return []validator.String{stringvalidator.LengthBetween(1, 512), modelPoolTrimmedString{}}
}

type modelPoolTrimmedString struct{}

func (modelPoolTrimmedString) Description(context.Context) string {
	return "Must not have leading or trailing whitespace."
}
func (v modelPoolTrimmedString) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v modelPoolTrimmedString) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && strings.TrimSpace(req.ConfigValue.ValueString()) != req.ConfigValue.ValueString() {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Model Pool Name", v.Description(ctx))
	}
}
