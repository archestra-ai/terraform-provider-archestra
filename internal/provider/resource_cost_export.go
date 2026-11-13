package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &CostExportResource{}

func NewCostExportResource() resource.Resource { return &CostExportResource{} }

// CostExportResource schedules FOCUS cost files to a blob provider. The API
// has no single-item read, so Read finds the export in the list.
type CostExportResource struct{ client *client.ClientWithResponses }

type CostExportResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	BlobProviderID types.String `tfsdk:"blob_provider_id"`
	Format         types.String `tfsdk:"format"`
	Schedule       types.String `tfsdk:"schedule"`
	PathPrefix     types.String `tfsdk:"path_prefix"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	StartFrom      types.String `tfsdk:"start_from"`
}

var costExportAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "blob_provider_id", JSONName: "blobProviderId", Kind: Scalar},
	{TFName: "format", JSONName: "format", Kind: Scalar},
	{TFName: "schedule", JSONName: "schedule", Kind: Scalar},
	{TFName: "path_prefix", JSONName: "pathPrefix", Kind: Scalar},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
	// Only creation takes a backfill start; the API never returns it.
	{TFName: "start_from", JSONName: "startFrom", Kind: Scalar, CreateOnly: true, OmitOnNull: true},
}

func (r *CostExportResource) AttrSpecs() []AttrSpec { return costExportAttrSpec }
func (r *CostExportResource) APIShape() any         { return client.ListCostExportsResponse{} }

// KnownIntentionallySkipped: audit timestamps and organization ownership are
// server bookkeeping. blobProviderName repeats the referenced provider. The
// rest is run state that changes with every run (exportedThrough, lastRunAt,
// lastRunStatus, lastRunError, lastObjectKey, running, nextRetryAt); surfacing it
// would rewrite state on every refresh.
func (r *CostExportResource) KnownIntentionallySkipped() []string {
	return []string{
		"organizationId", "createdAt", "updatedAt", "blobProviderName",
		"exportedThrough", "lastRunAt", "lastRunStatus", "lastRunError", "lastObjectKey", "running", "nextRetryAt",
	}
}

func (r *CostExportResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cost_export"
}

func (r *CostExportResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Scheduled export of LLM cost data as FOCUS CSV files to an `archestra_blob_provider` (API 1.5.0-rc.30 or later). " +
			"The platform writes a file per completed period; run state (last run, errors, retries) stays in the UI.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, MarkdownDescription: "Cost export UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":             schema.StringAttribute{Required: true, MarkdownDescription: "Display name.", Validators: []validator.String{stringvalidator.LengthBetween(1, 256), noSurroundingSpace()}},
			"blob_provider_id": schema.StringAttribute{Required: true, MarkdownDescription: "UUID of the `archestra_blob_provider` the files are written to.", Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical UUID.")}},
			"format":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("focus_csv"), MarkdownDescription: "File format. Only `focus_csv` today.", Validators: []validator.String{stringvalidator.OneOf("focus_csv")}},
			"schedule":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("daily"), MarkdownDescription: "How often a file is written: `hourly` or `daily`.", Validators: []validator.String{stringvalidator.OneOf("hourly", "daily")}},
			"path_prefix": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("cost-exports"),
				MarkdownDescription: "Key prefix for the export's files, under the blob provider's own prefix, without leading or trailing `/`.",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(512), noSurroundingSpace(),
					stringvalidator.RegexMatches(regexp.MustCompile(`^(?:[^/].*[^/]|[^/])?$`), "Must not start or end with /."),
				},
			},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether scheduled runs write files. Disable to pause the export."},
			"start_from": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "RFC 3339 UTC time (ending in `Z`) to export usage from, including the past (a backfill). Defaults to now. " +
					"Applies at creation only: changing it replaces the export. An imported export records the configured value without replacement.",
				Validators:    []validator.String{rfc3339Validator{}},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIf(startFromChanged, "Replace the export to apply a new start.", "Replace the export to apply a new start.")},
			},
		},
	}
}

func (r *CostExportResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got %T", req.ProviderData))
		return
	}
	r.client = api
}

func (r *CostExportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CostExportResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), costExportAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_cost_export Create", patch, costExportAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateCostExportWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Cost Export Creation Failed", apiFailure(result.StatusCode(), result.Body))
		return
	}
	flattenCostExport(&plan, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CostExportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CostExportResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.ID.IsNull() || state.ID.IsUnknown() || state.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	raw, found := r.find(ctx, state.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	flattenCostExport(&state, raw, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CostExportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior CostExportResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, costExportAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		// Only start_from changed on an imported export; there is nothing to send.
		plan.ID = prior.ID
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	LogPatch(ctx, "archestra_cost_export Update", patch, costExportAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	id, ok := parseUUID(prior.ID.ValueString(), &resp.Diagnostics)
	if !ok {
		return
	}
	result, err := r.client.UpdateCostExportWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Cost Export Update Failed", apiFailure(result.StatusCode(), result.Body))
		return
	}
	flattenCostExport(&plan, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CostExportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CostExportResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, ok := parseUUID(state.ID.ValueString(), &resp.Diagnostics)
	if !ok {
		return
	}
	result, err := r.client.DeleteCostExportWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) {
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Cost Export Deletion Failed", apiFailure(result.StatusCode(), result.Body))
	}
}

func (r *CostExportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *CostExportResource) find(ctx context.Context, id string, diags *diag.Diagnostics) (json.RawMessage, bool) {
	result, err := r.client.ListCostExportsWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return nil, false
	}
	if result.JSON200 == nil {
		diags.AddError("Cost Export Read Failed", apiFailure(result.StatusCode(), result.Body))
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(result.Body, &items); err != nil {
		diags.AddError("Invalid Cost Export Response", err.Error())
		return nil, false
	}
	for _, item := range items {
		var wire struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(item, &wire) == nil && wire.ID == id {
			return item, true
		}
	}
	return nil, false
}

// flattenCostExport sets every stored field from an export's JSON. start_from
// is not returned, so it keeps its current value.
func flattenCostExport(m *CostExportResourceModel, raw []byte, diags *diag.Diagnostics) {
	var wire struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		BlobProviderID string `json:"blobProviderId"`
		Format         string `json:"format"`
		Schedule       string `json:"schedule"`
		PathPrefix     string `json:"pathPrefix"`
		Enabled        bool   `json:"enabled"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		diags.AddError("Invalid Cost Export Response", err.Error())
		return
	}
	m.ID = types.StringValue(wire.ID)
	m.Name = types.StringValue(wire.Name)
	m.BlobProviderID = types.StringValue(wire.BlobProviderID)
	m.Format = types.StringValue(wire.Format)
	m.Schedule = types.StringValue(wire.Schedule)
	m.PathPrefix = types.StringValue(wire.PathPrefix)
	m.Enabled = types.BoolValue(wire.Enabled)
}

// startFromChanged replaces the export when an existing start changes. A null
// prior start (an imported export) takes the configured value in place.
func startFromChanged(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = !req.StateValue.IsNull() && !req.PlanValue.Equal(req.StateValue)
}

type rfc3339Validator struct{}

func (rfc3339Validator) Description(context.Context) string {
	return "must be an RFC 3339 UTC time ending in Z"
}
func (v rfc3339Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (rfc3339Validator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	// The API accepts UTC times only.
	value := req.ConfigValue.ValueString()
	if _, err := time.Parse(time.RFC3339, value); err != nil || !strings.HasSuffix(value, "Z") {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Time", "Use an RFC 3339 UTC time such as 2026-01-01T00:00:00Z.")
	}
}
